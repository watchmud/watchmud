package server

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"
	"uuid"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/world"
	"golang.org/x/crypto/bcrypt"
)

type GameServer struct {
	incomingBuffer chan *gameserver.HandlerParameter
	world          *world.World
	catalog        *rules.Catalog
	tickInterval   time.Duration
	store          player.Store
	// bcryptCost is bcrypt.DefaultCost; tests turn it down to MinCost.
	bcryptCost int
}

func New(w *world.World, c *rules.Catalog, s player.Store) *GameServer {
	const bufferSize = 64
	return &GameServer{
		incomingBuffer: make(chan *gameserver.HandlerParameter, bufferSize),
		world:          w,
		catalog:        c,
		store:          s,
		bcryptCost:     bcrypt.DefaultCost,
	}
}

// SetTickInterval is how often Run ticks; zero is rules.PulseInterval. Every
// pulse job counts pulses rather than reading a clock, so a faster tick runs
// the whole world faster and in the same order -- for tests that want a
// fight to finish while they wait.
func (gs *GameServer) SetTickInterval(d time.Duration) {
	gs.tickInterval = d
}

// Run the game server, obviously.
func (gs *GameServer) Run(ctx context.Context) error {
	log.Info().Msg("starting game server Run loop")

	interval := gs.tickInterval
	if interval == 0 {
		interval = rules.PulseInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	last := time.Now()
	var pulse rules.PulseCount

	for {
		select {
		case <-ctx.Done():
			gs.world.QueuePlayerRecords()
			return ctx.Err()
		case msg := <-gs.incomingBuffer:
			gs.recovering(msg, gs.dispatch)
			gs.prompt()
		case <-ticker.C:
			now := time.Now()
			delta := now.Sub(last)
			last = now
			pulse++
			gs.heartbeat(pulse, delta)
			gs.prompt()
		}
	}
}

// prompt tells every player the game is waiting on them again. Everyone, not
// just whoever sent the command: a say or a combat round lands on bystanders
// too, and they need their prompt back as much as the speaker does. The
// transport drops the prompts that nothing was said in front of, so this
// costs a channel send per player and prints nothing new to the rest.
//
// A player who has just logged in is in the list by now, so this is also
// their first prompt; one whose login failed is not, and gets none.
func (gs *GameServer) prompt() {
	for p := range gs.world.Players() {
		p.Send(event.Prompt{
			CurrentHealth: p.CurrentHealth(),
			MaxHealth:     p.MaxHealth(),
			CurrentMana:   p.CurrentMana(),
			MaxMana:       p.MaxMana(),
		})
	}
}

// Heartbeat runner of the game. Use pulse to determine intervals
// between things (ex. reset zones every 15 minutes...)
// delta is the amount of time since the last heartbeat was run.
func (gs *GameServer) heartbeat(pulse rules.PulseCount, delta time.Duration) {
	//log.Debug().Msgf("pulse %d hb %d", pulse, delta)

	// pulse zone
	// (zone reset ...)
	zonePulse := gs.catalog.MudTime.Zone
	if pulse.CheckInterval(zonePulse) {
		recoverPulse("zone", gs.world.DoZoneActivity)
	}

	// pulse mobs
	// (mobs walk around, initiate attack?)
	mobPulse := gs.catalog.MudTime.Mobile
	if pulse.CheckInterval(mobPulse) {
		recoverPulse("mobile", gs.world.DoMobileActivity)
		// on the same pulse: CorpseDecay and DroppedDecay are minutes, and
		// ten seconds late is nothing anyone will notice
		recoverPulse("floors", gs.world.DecayFloors)
	}

	// perform violence
	// do the attacking (players and mobs and everybody)
	violencePulse := gs.catalog.MudTime.Violence
	if pulse.CheckInterval(violencePulse) {
		recoverPulse("violence", func() { gs.world.DoViolence(pulse) })
	}

	// anyone not fighting gets some health back
	regenPulse := gs.catalog.MudTime.Regen
	if pulse.CheckInterval(regenPulse) {
		recoverPulse("regen", gs.world.Regenerate)
	}

	// saving player data
	savePulse := gs.catalog.MudTime.PlayerSave
	if pulse.CheckInterval(savePulse) {
		recoverPulse("save", gs.world.QueuePlayerRecords)
	}
}

// dispatch a message to its handler.
//
// The pre-world cases -- login, create player -- are handled here because
// they need the store and player construction. Everything else falls through
// to the world.
func (gs *GameServer) dispatch(msg *gameserver.HandlerParameter) error {
	switch cmd := msg.Command.(type) {
	case command.Login:
		return gs.handleLogin(msg, cmd)
	case command.CreatePlayer:
		return gs.handleCreatePlayer(msg, cmd)
	case loginChecked:
		return gs.handleLoginChecked(msg, cmd)
	case createHashed:
		return gs.handleCreateHashed(msg, cmd)
	default:
		return gs.world.HandleIncomingMessage(msg)
	}
}

func (gs *GameServer) Receive(msg *gameserver.HandlerParameter) {
	gs.incomingBuffer <- msg
}

func (gs *GameServer) Logout(c gameserver.Conn, cause string) {
	gs.Receive(gameserver.NewHandlerParameter(c, command.Logout{Cause: cause}))
}

func (gs *GameServer) handleLogin(msg *gameserver.HandlerParameter, cmd command.Login) error {
	// is this connection already authenticated?
	// see if we can find an existing player.
	if msg.Client.Player() != nil {
		// you've already got one - this is an error in our connection logic
		return errors.New("player already attached to client")
	}

	// Every name is stored in one form, so the lookups below can be exact
	// and "BOB" is Bob. Something that can't be a name is refused before a
	// password or a creation is offered for it.
	playerName, err := player.CanonicalName(cmd.Name)
	if err != nil {
		msg.Client.Send(event.LoginFailed{Reason: event.InvalidName})
		return nil
	}
	// One character, one session. A second one would load its own copy of
	// the character, and the two would take turns saving over each other --
	// drop a sword in one and the other still has it to save back. Taking
	// over the old session is the friendlier answer; refusing is the safe one.
	if gs.world.IsPlaying(playerName) {
		msg.Client.Send(event.LoginFailed{Reason: event.AlreadyPlaying})
		return nil
	}
	rec, found, err := gs.store.Load(playerName)
	if err != nil {
		// store error - problem with the store, return an error
		return err
	}
	if !found {
		// not an error - could represent a new player (player creation),
		// unless the world has the word already
		if gs.world.IsReservedName(playerName) {
			msg.Client.Send(event.LoginFailed{Reason: event.NameReserved})
			return nil
		}
		log.Info().Str("playerName", playerName).Msg("playerName not found in store")
		msg.Client.Send(event.LoginFailed{Reason: event.NoSuchPlayer})
		return nil
	}
	if cmd.Password == "" {
		msg.Client.Send(event.LoginFailed{Reason: event.PasswordRequired})
		return nil
	}

	go func() {
		ok := bcrypt.CompareHashAndPassword([]byte(rec.PasswordHash), []byte(cmd.Password)) == nil
		gs.Receive(gameserver.NewHandlerParameter(msg.Client, loginChecked{Name: playerName, Ok: ok}))
	}()
	// return to handleLoginChecked
	return nil
}

func (gs *GameServer) handleLoginChecked(msg *gameserver.HandlerParameter, cmd loginChecked) error {
	if !cmd.Ok {
		log.Warn().Msgf("login failed for %s", cmd.Name)
		msg.Client.Send(event.LoginFailed{Reason: event.BadPassword})
		return nil
	}
	// have to check again because something might have gotten through
	if msg.Client.Player() != nil {
		// you've already got one - this is an error in our connection logic
		return errors.New("player already attached to client")
	}

	// One character, one session. A second one would load its own copy of
	// the character, and the two would take turns saving over each other --
	// drop a sword in one and the other still has it to save back. Taking
	// over the old session is the friendlier answer; refusing is the safe one.
	if gs.world.IsPlaying(cmd.Name) {
		msg.Client.Send(event.LoginFailed{Reason: event.AlreadyPlaying})
		return nil
	}
	// reload record
	rec, found, err := gs.store.Load(cmd.Name)
	if err != nil {
		// store error - problem with the store, return an error
		return err
	}
	if !found {
		// shouldn't happen unless something crazy with database
		// not an error - could represent a new player (player creation)
		log.Info().Str("playerName", cmd.Name).Msg("playerName not found in store")
		msg.Client.Send(event.LoginFailed{Reason: event.NoSuchPlayer})
		return nil
	}

	// turn the record into a player.Player
	p, err := player.FromRecord(rec, msg.Client, gs.catalog, gs.world)
	if err != nil {
		return fmt.Errorf("handleLogin %s: %w", cmd.Name, err)
	}
	msg.Player = p
	msg.Client.SetPlayer(p)

	// back where they left off
	gs.world.ReturnPlayer(p, rec.LastZoneId, rec.LastRoomId)

	p.Send(event.LoggedIn{Name: p.Name()})
	gs.world.Arrive(p)
	return nil
}

func (gs *GameServer) handleCreatePlayer(msg *gameserver.HandlerParameter, cmd command.CreatePlayer) error {
	if msg.Client.Player() != nil {
		// you've already got one
		// this is a programming bug (login state machine), so report the error
		return fmt.Errorf("player %s already attached to client", msg.Client.Player().Name())
	}
	playerName, err := player.CanonicalName(cmd.Name)
	if err != nil {
		msg.Client.Send(event.CreateFailed{Reason: event.InvalidName})
		return nil
	}
	if len(cmd.Password) == 0 {
		msg.Client.Send(event.CreateFailed{Reason: event.BadRequest})
		return fmt.Errorf("handleCreatePlayer %s: %s", playerName, event.BadRequest)
	}
	if gs.world.IsReservedName(playerName) {
		msg.Client.Send(event.CreateFailed{Reason: event.NameReserved})
		return nil
	}

	// The name has to be checked here. It used to be the database's unique
	// index that refused a duplicate, but saves are queued now and that error
	// only reaches the writer goroutine, long after both characters are
	// playing. Load sees the queue as well as the database, and dispatch is
	// one command at a time, so this cannot race another creation.
	if _, taken, err := gs.store.Load(playerName); err != nil {
		return fmt.Errorf("handleCreatePlayer %s: %w", playerName, err)
	} else if taken {
		msg.Client.Send(event.CreateFailed{Reason: event.NameTaken})
		return nil
	}

	// The lineage is the only choice creation makes, and it is cosmetic. An
	// id the catalog doesn't know means the transport offered something stale
	// -- worth a log line, not worth refusing to make the character.
	lineage, found := gs.catalog.Lineages[cmd.Lineage]
	if !found {
		if cmd.Lineage != "" {
			log.Warn().Str("playerName", playerName).Msgf("unknown lineage %q at creation, using the default", cmd.Lineage)
		}
		lineage = gs.catalog.DefaultLineage()
	}
	if lineage == nil {
		return errors.New("handleCreatePlayer: no lineages defined in the catalog")
	}

	// separate go func because bcrypt is slooooow and can't be on the gameserver's goroutine.
	// its ok because this doesn't modify any game / world state.
	go func() {
		hash, err := bcrypt.GenerateFromPassword([]byte(cmd.Password), gs.bcryptCost)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to hash password?! Bad bad not good.")
			msg.Client.Send(event.CreateFailed{Reason: event.Unknown})
			return
		}
		gs.Receive(gameserver.NewHandlerParameter(msg.Client, createHashed{
			Name:         playerName,
			Lineage:      cmd.Lineage,
			HashPassword: command.Secret(hash),
		}))
	}()
	// control goes to handleCreateHashed
	return nil
}

func (gs *GameServer) handleCreateHashed(msg *gameserver.HandlerParameter, cmd createHashed) error {
	// Checked again: another connection could have created this name while
	// the hash was being made.
	if _, taken, err := gs.store.Load(cmd.Name); err != nil {
		return fmt.Errorf("handleCreateHashed %s: %w", cmd.Name, err)
	} else if taken {
		msg.Client.Send(event.CreateFailed{Reason: event.NameTaken})
		return nil
	}
	// The lineage is the only choice creation makes, and it is cosmetic. An
	// id the catalog doesn't know means the transport offered something stale
	// -- worth a log line, not worth refusing to make the character.
	lineage, found := gs.catalog.Lineages[cmd.Lineage]
	if !found {
		if cmd.Lineage != "" {
			log.Warn().Str("playerName", cmd.Name).Msgf("unknown lineage %q at creation, using the default", cmd.Lineage)
		}
		lineage = gs.catalog.DefaultLineage()
	}
	if lineage == nil {
		return errors.New("handleCreateHashed: no lineages defined in the catalog")
	}

	p := player.New(
		uuid.New(),
		cmd.Name,
		string(cmd.HashPassword),
		msg.Client,
		lineage,
		gs.catalog,
	)

	// The kit goes on before the save, so the first thing written for this
	// character already has it: a crash between here and the first command
	// leaves them dressed rather than naked.
	player.GiveStartingGear(p, gs.catalog.StartingGear, gs.world)

	if err := gs.store.Save(p.Record()); err != nil {
		return fmt.Errorf("handleCreateHashed: %v", err)
	}

	msg.Client.SetPlayer(p)
	msg.Player = p

	gs.world.PlacePlayer(p, gs.world.StartRoom)

	p.Send(event.PlayerCreated{Name: p.Name()})
	gs.world.Arrive(p)
	gs.world.Welcome(p)
	return nil
}

// recovering runs one message's handler and survives it panicking. Without
// this, one bad handler -- a nil room, an index out of range -- ends the
// process and every player's session with it. The world may be left
// half-changed by whatever the handler got through before it died, which is
// still better than no world; the stack goes to the log so it gets fixed.
//
// Whoever sent it is told something went wrong. Before login that means a
// failed login: the login conversation is blocked waiting for an answer, and
// would otherwise wait until the idle timeout.
func (gs *GameServer) recovering(msg *gameserver.HandlerParameter, handle func(*gameserver.HandlerParameter) error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		log.Error().
			Str("command", fmt.Sprintf("%T", msg.Command)).
			Str("stack", string(debug.Stack())).
			Msgf("panic handling a command: %v", r)
		if p := msg.Client.Player(); p != nil {
			verb := ""
			if msg.Command != nil {
				verb = msg.Command.Verb()
			}
			p.Send(event.Failed{Verb: verb, Code: event.Unknown})
		} else {
			msg.Client.Send(event.LoginFailed{Reason: event.Unknown})
		}
	}()
	if err := handle(msg); err != nil {
		// don't return error, we're not halting the server
		log.Error().Err(err).Msg("error dispatching message")
	}
}

// recoverPulse runs one heartbeat job and survives it panicking, on its own
// so that the jobs after it still run: the save is last, and a crash in
// combat every pulse must not also stop everybody being saved.
func recoverPulse(job string, run func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().
				Str("pulse", job).
				Str("stack", string(debug.Stack())).
				Msgf("panic in a pulse: %v", r)
		}
	}()
	run()
}
