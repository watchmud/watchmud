package server

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync/atomic"
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
	// bcryptSlots is how many hashes run at once: one a CPU. Each is
	// deliberately slow, and a crowd of logins mustn't take every core from
	// the world goroutine.
	bcryptSlots chan struct{}
	// logouts counts each name's logouts since start, and creating holds
	// the names being created: both world-goroutine only, for the checks a
	// login or a creation makes after its store lookup comes back.
	logouts  map[string]int
	creating map[string]bool
	// inFlight is the connections with a lookup or a hash away on a
	// goroutine, and gone those of them that hung up meanwhile.
	inFlight map[gameserver.Conn]bool
	gone     map[gameserver.Conn]bool
	// lastBeat is unix nanoseconds at the end of the last tick, for the
	// health check (health.go); zero until Run starts.
	lastBeat atomic.Int64
	// playing is how many people -- bots not counted -- were in the world
	// at the last prompt, for MSSP; started is when New ran. Both safe to
	// read from another goroutine.
	playing atomic.Int64
	started time.Time
}

func New(w *world.World, c *rules.Catalog, s player.Store) *GameServer {
	const bufferSize = 64
	return &GameServer{
		incomingBuffer: make(chan *gameserver.HandlerParameter, bufferSize),
		world:          w,
		catalog:        c,
		store:          s,
		bcryptCost:     bcrypt.DefaultCost,
		bcryptSlots:    make(chan struct{}, runtime.GOMAXPROCS(0)),
		logouts:        map[string]int{},
		creating:       map[string]bool{},
		inFlight:       map[gameserver.Conn]bool{},
		gone:           map[gameserver.Conn]bool{},
		started:        time.Now(),
	}
}

// Playing is how many people were in the world at the last prompt, bots
// not counted: what a listing site is told.
func (gs *GameServer) Playing() int { return int(gs.playing.Load()) }

// Started is when the server came up.
func (gs *GameServer) Started() time.Time { return gs.started }

// SetPasswordCost is bcrypt's cost for passwords made and checked from here
// on, for a server whose characters are throwaway -- tests and the load test,
// which would otherwise spend most of their time hashing. Never production.
func (gs *GameServer) SetPasswordCost(cost int) { gs.bcryptCost = cost }

// SetTickInterval is how often Run ticks; zero is rules.PulseInterval. Every
// pulse job counts pulses rather than reading a clock, so a faster tick runs
// the whole world faster and in the same order -- for tests that want a
// fight to finish while they wait.
//
// The world's clock keeps the same pace, so cooldowns -- which read a clock,
// not pulses -- are as much faster as the rest.
func (gs *GameServer) SetTickInterval(d time.Duration) {
	gs.tickInterval = d
	if d > 0 {
		gs.world.SetPace(float64(rules.PulseInterval) / float64(d))
	}
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
	gs.beat(last)
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
			gs.beat(time.Now())
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
	people := 0
	defer func() { gs.playing.Store(int64(people)) }()
	for p := range gs.world.Players() {
		if !p.IsBot() {
			people++
		}
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

	// scripts paused on wait(): every pulse, and before violence, so a hook
	// that carries on into me:summon has its summons in for this round
	recoverPulse("scripts", gs.world.ResumeScripts)

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
	case loginLooked:
		if gs.hungUp(msg.Client) {
			return nil
		}
		return gs.handleLoginLooked(msg, cmd)
	case loginChecked:
		if gs.hungUp(msg.Client) {
			return nil
		}
		return gs.handleLoginChecked(msg, cmd)
	case command.Logout:
		// The parameter's player was snapshotted when the Logout was built,
		// on the connection's goroutine -- maybe before a login queued ahead
		// of it attached one. The connection's own is the one to log out, or
		// a nil snapshot leaves the just-logged-in character a ghost.
		msg.Player = msg.Client.Player()
		// a character's record changes as they leave: a login still being
		// checked for them loads it again (handleLoginChecked)
		if p := msg.Player; p != nil {
			gs.logouts[p.Name()]++
		} else if gs.inFlight[msg.Client] {
			// gone mid-login: whatever comes back for it is dropped, or the
			// character would be in the world with nobody there to play
			// them -- and no Logout ever to come
			gs.gone[msg.Client] = true
		}
		return gs.world.HandleIncomingMessage(msg)
	case createHashed:
		if gs.hungUp(msg.Client) {
			delete(gs.creating, cmd.Name)
			return nil
		}
		return gs.handleCreateHashed(msg, cmd)
	default:
		return gs.world.HandleIncomingMessage(msg)
	}
}

// hungUp takes note that c's lookup or hash has come back, and answers
// whether c hung up while it was away; if so, the answer is for nobody.
func (gs *GameServer) hungUp(c gameserver.Conn) bool {
	delete(gs.inFlight, c)
	if gs.gone[c] {
		delete(gs.gone, c)
		return true
	}
	return false
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
	gs.lookUp(msg.Client, playerName, cmd.Password)
	// on to handleLoginLooked
	return nil
}

// lookUp loads name's record on a goroutine of its own and hands it back to
// the world as loginLooked. The store is a database: on the world goroutine
// a slow answer -- mongo gives up after five seconds -- would stop the game
// for everyone, once for every name typed at the door.
func (gs *GameServer) lookUp(c gameserver.Conn, name string, password command.Secret) {
	gen := gs.logouts[name]
	gs.inFlight[c] = true
	go func() {
		rec, found, err := gs.store.Load(name)
		gs.Receive(gameserver.NewHandlerParameter(c, loginLooked{
			Name: name, Password: password, Rec: rec, Found: found, Err: err, Gen: gen,
		}))
	}()
}

func (gs *GameServer) handleLoginLooked(msg *gameserver.HandlerParameter, cmd loginLooked) error {
	if cmd.Err != nil {
		// store error - problem with the store; recovering answers the login
		return fmt.Errorf("handleLogin %s: %w", cmd.Name, cmd.Err)
	}
	if !cmd.Found {
		// not an error - could represent a new player (player creation),
		// unless the world has the word already
		if gs.world.IsReservedName(cmd.Name) {
			msg.Client.Send(event.LoginFailed{Reason: event.NameReserved})
			return nil
		}
		log.Debug().Str("playerName", cmd.Name).Msg("playerName not found in store")
		msg.Client.Send(event.LoginFailed{Reason: event.NoSuchPlayer})
		return nil
	}
	if cmd.Password == "" {
		msg.Client.Send(event.LoginFailed{Reason: event.PasswordRequired})
		return nil
	}

	rec := cmd.Rec
	gs.inFlight[msg.Client] = true
	go func() {
		gs.bcryptSlots <- struct{}{}
		defer func() { <-gs.bcryptSlots }()
		ok := bcrypt.CompareHashAndPassword([]byte(rec.PasswordHash), []byte(cmd.Password)) == nil
		gs.Receive(gameserver.NewHandlerParameter(msg.Client, loginChecked{Name: cmd.Name, Ok: ok, Rec: rec, Gen: cmd.Gen}))
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
	// The record was loaded before the password was checked. If the
	// character has logged out since -- another session, quitting with
	// newer gear -- it's stale: load it again, off this goroutine, and come
	// back here, the password already known good.
	if gs.logouts[cmd.Name] != cmd.Gen {
		gen := gs.logouts[cmd.Name]
		gs.inFlight[msg.Client] = true
		go func() {
			rec, found, err := gs.store.Load(cmd.Name)
			if err != nil || !found {
				// handleLoginLooked answers either, as at the start
				gs.Receive(gameserver.NewHandlerParameter(msg.Client, loginLooked{Name: cmd.Name, Found: found, Err: err, Gen: gen}))
				return
			}
			gs.Receive(gameserver.NewHandlerParameter(msg.Client, loginChecked{Name: cmd.Name, Ok: true, Rec: rec, Gen: gen}))
		}()
		return nil
	}
	rec := cmd.Rec

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
		// answered here, so no error for recovering to answer a second time
		log.Warn().Str("playerName", playerName).Msg("handleCreatePlayer: no password")
		msg.Client.Send(event.CreateFailed{Reason: event.BadRequest})
		return nil
	}
	if gs.world.IsReservedName(playerName) {
		msg.Client.Send(event.CreateFailed{Reason: event.NameReserved})
		return nil
	}

	// The name has to be checked here. It used to be the database's unique
	// index that refused a duplicate, but saves are queued now and that error
	// only reaches the writer goroutine, long after both characters are
	// playing. The store is asked on the creation's goroutine; a second
	// creation of the name meanwhile is refused here, by creating, which only
	// the world goroutine touches.
	if gs.creating[playerName] {
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

	gs.creating[playerName] = true // until handleCreateHashed
	gs.inFlight[msg.Client] = true

	// separate go func because the store is a database and bcrypt is
	// slooooow, and neither can be on the gameserver's goroutine. It's ok
	// because this doesn't modify any game / world state.
	go func() {
		done := createHashed{Name: playerName, Lineage: cmd.Lineage}
		defer func() { gs.Receive(gameserver.NewHandlerParameter(msg.Client, done)) }()
		if _, taken, err := gs.store.Load(playerName); err != nil || taken {
			done.Taken, done.Err = taken, err
			return
		}
		gs.bcryptSlots <- struct{}{}
		defer func() { <-gs.bcryptSlots }()
		hash, err := bcrypt.GenerateFromPassword([]byte(cmd.Password), gs.bcryptCost)
		if err != nil {
			done.Err = fmt.Errorf("hashing a password: %w", err)
			return
		}
		done.HashPassword = command.Secret(hash)
	}()
	// control goes to handleCreateHashed
	return nil
}

func (gs *GameServer) handleCreateHashed(msg *gameserver.HandlerParameter, cmd createHashed) error {
	// Whatever happened, the name is free to try again: the record is saved
	// (and queued, so the store's Load sees it) before anyone could.
	delete(gs.creating, cmd.Name)
	if cmd.Err != nil {
		// recovering answers the conversation
		return fmt.Errorf("handleCreateHashed %s: %w", cmd.Name, cmd.Err)
	}
	if cmd.Taken {
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
	gs.world.ArriveNew(p)
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
		switch msg.Command.(type) {
		case loginChecked, createHashed:
			// A login's last step: the conversation is waiting on an answer
			// that would never come, even if the player was already
			// attached. Undo the half-done login and answer it.
			if p := msg.Client.Player(); p != nil {
				if gs.world.IsPlaying(p.Name()) {
					gs.world.RemovePlayer(p)
				}
				msg.Client.SetPlayer(nil)
			}
			msg.Client.Send(event.LoginFailed{Reason: event.Unknown})
			return
		}
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
		// but answer a login: the connection is waiting on one, and would
		// wait for good -- holding its address's slot -- on a store hiccup
		if msg.Client.Player() == nil {
			msg.Client.Send(event.LoginFailed{Reason: event.Unknown})
		}
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
