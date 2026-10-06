package server

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/memstore"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/testdice"
	"github.com/watchmud/watchmud/world"
	"golang.org/x/crypto/bcrypt"
)

// testConn is a connection with nobody on it yet, which is what creation
// starts from. It records rather than forwarding to the player, since the
// player it is about to be given sends through this same conn.
type testConn struct {
	p    *player.Player
	sent []any
}

func (c *testConn) Send(msg any)               { c.sent = append(c.sent, msg) }
func (c *testConn) Player() *player.Player     { return c.p }
func (c *testConn) SetPlayer(p *player.Player) { c.p = p }
func (c *testConn) Close()                     {}

func newTestGameServer(t *testing.T) (*GameServer, *memstore.Store) {
	t.Helper()
	return newTestGameServerRolling(t, testdice.New())
}

// newTestGameServerRolling is newTestGameServer with dice the test loads.
func newTestGameServerRolling(t *testing.T, dice *testdice.LoadedDice) (*GameServer, *memstore.Store) {
	t.Helper()

	content, err := loader.LoadContent(os.DirFS("../testcontent"))
	require.NoError(t, err)

	store := memstore.New()
	w, err := world.New(content, store, dice)
	require.NoError(t, err)

	gs := New(w, content.Catalog, store)
	gs.bcryptCost = bcrypt.MinCost
	return gs, store
}

// settle does what Run would: takes the callback a login or creation queued
// from its bcrypt goroutine and dispatches it.
func settle(t *testing.T, gs *GameServer) {
	t.Helper()
	select {
	case msg := <-gs.incomingBuffer:
		require.NoError(t, gs.dispatch(msg))
	case <-time.After(5 * time.Second):
		t.Fatal("nothing came back from the bcrypt goroutine")
	}
}

// answers counts what the login conversation waits on: the four events it
// ends on.
func answers(c *testConn) int {
	n := 0
	for _, m := range c.sent {
		switch m.(type) {
		case event.LoggedIn, event.PlayerCreated, event.LoginFailed, event.CreateFailed:
			n++
		}
	}
	return n
}

// ask sends cmd from c and does what Run would until c has its answer: the
// store lookup and the hashing each come back through the queue.
func ask(t *testing.T, gs *GameServer, c *testConn, cmd command.Command) {
	t.Helper()
	before := answers(c)
	require.NoError(t, gs.dispatch(gameserver.NewHandlerParameter(c, cmd)))
	for answers(c) == before {
		settle(t, gs)
	}
}

// create and login are the whole conversation.
func create(t *testing.T, gs *GameServer, c *testConn, name, password string) {
	t.Helper()
	ask(t, gs, c, command.CreatePlayer{Name: name, Lineage: "human", Password: command.Secret(password)})
}

func login(t *testing.T, gs *GameServer, c *testConn, name, password string) {
	t.Helper()
	ask(t, gs, c, command.Login{Name: name, Password: command.Secret(password)})
}

// A brand new character arrives dressed, and the record written for them says
// so -- so the gear is still there on their next login.
// create player is two steps now because of hashing the password.
func TestCreatePlayer_startingGear(t *testing.T) {
	gs, store := newTestGameServer(t)
	c := &testConn{}

	create(t, gs, c, "newbie", "sekrit")

	p := c.Player()
	require.NotNil(t, p)
	// created, then shown where they are -- in that order, so the telnet login
	// conversation is over before the description arrives -- then told where
	// to go from there. Their color setting goes ahead of the room.
	require.Len(t, c.sent, 4)
	assert.IsType(t, event.PlayerCreated{}, c.sent[0])
	assert.Equal(t, event.Color{On: true}, c.sent[1])
	assert.IsType(t, event.RoomDescription{}, c.sent[2])
	assert.Equal(t, event.Welcome{Text: "Welcome! The fighting is south."}, c.sent[3])

	// testcontent's kit: a knife, a helmet, a rope that is only carried,
	// and the temple token worn on the neck.
	assert.Equal(t, 4, p.Inventory().Len())

	knife := p.Equipment().At(rules.SlotWield)
	require.NotNil(t, knife)
	assert.Equal(t, "knife", knife.Definition.Name)
	require.NotNil(t, p.Equipment().At(rules.SlotHead))

	rec, found, err := store.Load("Newbie")
	require.NoError(t, err)
	require.True(t, found)
	assert.Len(t, rec.Inventory, 4)
	assert.Len(t, rec.Equipment, 3)

	// and it comes back the same way it went in
	restored, err := player.FromRecord(rec, &player.Recorder{}, gs.catalog, gs.world)
	require.NoError(t, err)
	assert.Equal(t, 4, restored.Inventory().Len())
	require.NotNil(t, restored.Equipment().At(rules.SlotWield))
	require.NotNil(t, restored.Equipment().At(rules.SlotHead))
}

// Once a character is in the world, the prompt reaches them -- which is what
// puts the first "> " on the screen after login.
func TestPrompt_reachesPlayersInTheWorld(t *testing.T) {
	gs, _ := newTestGameServer(t)
	c := &testConn{}
	create(t, gs, c, "newbie", "sekrit")

	c.Player().TakeMeleeDamage(30)
	gs.prompt()

	assert.Equal(t, event.Prompt{CurrentHealth: 70, MaxHealth: 100, CurrentMana: 100, MaxMana: 100}, c.sent[len(c.sent)-1])
}

// A failed login never joined the world, so the login conversation isn't
// interrupted by a prompt it didn't ask for.
func TestPrompt_skipsAFailedLogin(t *testing.T) {
	gs, _ := newTestGameServer(t)
	c := &testConn{}
	ask(t, gs, c, command.Login{Name: "nobody"})

	gs.prompt()

	require.Len(t, c.sent, 1)
	assert.IsType(t, event.LoginFailed{}, c.sent[0])
}

// A returning player comes back where they left off.
func TestLogin_returnsToTheLastRoom(t *testing.T) {
	gs, store := newTestGameServer(t)
	c := &testConn{}
	create(t, gs, c, "wanderer", "sekrit")
	rec, _, err := store.Load("Wanderer")
	require.NoError(t, err)
	gs.world.RemovePlayer(c.Player())

	rec.LastZoneId, rec.LastRoomId = "wrathrock", "market_square"
	require.NoError(t, store.Save(rec))

	back := &testConn{}
	login(t, gs, back, "wanderer", "sekrit")

	require.NotNil(t, back.Player())
	require.Len(t, back.sent, 3, "no welcome for a returning player")
	assert.IsType(t, event.LoggedIn{}, back.sent[0])
	assert.IsType(t, event.Color{}, back.sent[1])
	assert.Equal(t, "Market Square", back.sent[2].(event.RoomDescription).Name, "shown where they came back to")

	// a look is saved like any other command, and the save says where they are
	require.NoError(t, gs.dispatch(gameserver.NewHandlerParameter(back, command.Look{})))
	rec, _, err = store.Load("Wanderer")
	require.NoError(t, err)
	assert.Equal(t, "market_square", rec.LastRoomId)
}

// The store no longer refuses a duplicate name itself (saves are queued), so
// creation has to.
func TestCreatePlayer_nameTaken(t *testing.T) {
	gs, _ := newTestGameServer(t)
	create(t, gs, &testConn{}, "bob", "sekrit")

	second := &testConn{}
	ask(t, gs, second, command.CreatePlayer{Name: "bob", Password: "other"})

	assert.Nil(t, second.Player(), "no second bob")
	require.Len(t, second.sent, 1)
	assert.Equal(t, event.CreateFailed{Reason: event.NameTaken}, second.sent[0])
}

// One character, one session: a second login is refused while the first is
// playing, and allowed once they have left.
func TestLogin_alreadyPlaying(t *testing.T) {
	gs, _ := newTestGameServer(t)
	first := &testConn{}
	create(t, gs, first, "bob", "sekrit")

	second := &testConn{}
	ask(t, gs, second, command.Login{Name: "bob", Password: "sekrit"})
	assert.Nil(t, second.Player())
	require.Len(t, second.sent, 1)
	assert.Equal(t, event.LoginFailed{Reason: event.AlreadyPlaying}, second.sent[0])

	require.NoError(t, gs.dispatch(gameserver.NewHandlerParameter(first, command.Logout{})))

	third := &testConn{}
	login(t, gs, third, "bob", "sekrit")
	assert.NotNil(t, third.Player(), "back in once the first session is gone")
}

// The wrong password is refused, with a reason: login() reads an empty one as
// success.
func TestLogin_wrongPassword(t *testing.T) {
	gs, _ := newTestGameServer(t)
	first := &testConn{}
	create(t, gs, first, "bob", "sekrit")
	require.NoError(t, gs.dispatch(gameserver.NewHandlerParameter(first, command.Logout{})))

	c := &testConn{}
	login(t, gs, c, "bob", "guess")

	assert.Nil(t, c.Player())
	require.Len(t, c.sent, 1)
	assert.Equal(t, event.LoginFailed{Reason: event.BadPassword}, c.sent[0])
}

// Two creations of one name can both pass the first check while their hashes
// are being made; the second to come back must lose.
func TestCreatePlayer_nameTakenWhileHashing(t *testing.T) {
	gs, _ := newTestGameServer(t)
	first, second := &testConn{}, &testConn{}
	require.NoError(t, gs.dispatch(gameserver.NewHandlerParameter(first, command.CreatePlayer{Name: "bob", Password: "sekrit"})))
	ask(t, gs, second, command.CreatePlayer{Name: "bob", Password: "sekrit"})
	assert.Equal(t, []any{event.CreateFailed{Reason: event.NameTaken}}, second.sent, "the first is still being made")

	for first.Player() == nil {
		settle(t, gs)
	}
	assert.Nil(t, second.Player())
}

// A name sent without a password is how the login conversation finds out
// whether to ask for one or offer to create the character. No bcrypt for it:
// there's nothing to compare.
func TestLogin_noPasswordAsksForOne(t *testing.T) {
	gs, _ := newTestGameServer(t)
	first := &testConn{}
	create(t, gs, first, "bob", "sekrit")
	require.NoError(t, gs.dispatch(gameserver.NewHandlerParameter(first, command.Logout{})))

	c := &testConn{}
	ask(t, gs, c, command.Login{Name: "bob"})

	assert.Nil(t, c.Player())
	assert.Equal(t, []any{event.LoginFailed{Reason: event.PasswordRequired}}, c.sent)
	assert.Empty(t, gs.incomingBuffer, "nothing went off to bcrypt")
}

// A name is stored one way and found in any case, so bob and BOB are one
// character -- and the store's unique index enforces it without knowing why.
func TestName_caseFolded(t *testing.T) {
	gs, store := newTestGameServer(t)
	first := &testConn{}
	create(t, gs, first, "bOB", "sekrit")
	require.NotNil(t, first.Player())
	assert.Equal(t, "Bob", first.Player().Name())

	rec, found, err := store.Load("Bob")
	require.NoError(t, err)
	require.True(t, found, "stored under the canonical name")
	assert.Equal(t, "Bob", rec.Name)

	// same character, whatever the case: taken to a creation, playing to a login
	dup := &testConn{}
	ask(t, gs, dup, command.CreatePlayer{Name: "BOB", Password: "other"})
	assert.Equal(t, []any{event.CreateFailed{Reason: event.NameTaken}}, dup.sent)

	second := &testConn{}
	ask(t, gs, second, command.Login{Name: "bob", Password: "sekrit"})
	assert.Equal(t, []any{event.LoginFailed{Reason: event.AlreadyPlaying}}, second.sent)

	require.NoError(t, gs.dispatch(gameserver.NewHandlerParameter(first, command.Logout{})))
	back := &testConn{}
	login(t, gs, back, "BOB", "sekrit")
	require.NotNil(t, back.Player())
	assert.Equal(t, "Bob", back.Player().Name())
}

// A name that can't be one is refused at the name prompt, before a password
// or a creation is offered for it.
func TestName_invalid(t *testing.T) {
	gs, store := newTestGameServer(t)
	for _, name := range []string{"bo", "bob2", "bob the great", ""} {
		c := &testConn{}
		ask(t, gs, c, command.Login{Name: name})
		assert.Equal(t, []any{event.LoginFailed{Reason: event.InvalidName}}, c.sent, "login %q", name)

		c = &testConn{}
		ask(t, gs, c, command.CreatePlayer{Name: name, Password: "sekrit"})
		assert.Equal(t, []any{event.CreateFailed{Reason: event.InvalidName}}, c.sent, "create %q", name)
		assert.Empty(t, gs.incomingBuffer, "nothing went to bcrypt for %q", name)
	}
	_, found, err := store.Load("Bob2")
	require.NoError(t, err)
	assert.False(t, found)
}

// Words the world already uses -- the target grammar, and every mob -- can't
// be a new character's name.
func TestName_reserved(t *testing.T) {
	gs, _ := newTestGameServer(t)
	for _, name := range []string{"self", "All", "rabbit", "drone"} {
		c := &testConn{}
		ask(t, gs, c, command.Login{Name: name})
		assert.Equal(t, []any{event.LoginFailed{Reason: event.NameReserved}}, c.sent, "login %q", name)

		c = &testConn{}
		ask(t, gs, c, command.CreatePlayer{Name: name, Password: "sekrit"})
		assert.Equal(t, []any{event.CreateFailed{Reason: event.NameReserved}}, c.sent, "create %q", name)
	}
}

// Pulse work is counted in pulses, not seconds, so a faster ticker speeds the
// whole world up together -- which is how bot's test fights a goose in well
// under a second. Regen is every 5 pulses; at 1ms a tick, 200ms is 200 pulses.
func TestRun_tickIntervalSetsThePace(t *testing.T) {
	gs, _ := newTestGameServer(t)
	c := &testConn{}
	create(t, gs, c, "newbie", "sekrit")
	c.Player().TakeMeleeDamage(30)

	gs.SetTickInterval(time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = gs.Run(ctx) // returns when ctx expires; the world is ours again after

	assert.Greater(t, c.Player().CurrentHealth(), 70)
}

// Scripts carry on every pulse, before violence: the chanter's imp arrives
// and swings in the same heartbeat.
func TestHeartbeat_resumesScriptsBeforeViolence(t *testing.T) {
	dice := testdice.New()
	gs, _ := newTestGameServerRolling(t, dice)
	c := &testConn{}
	create(t, gs, c, "newbie", "sekrit")
	for range 3 { // temple square to the road outside the south gate
		require.NoError(t, gs.dispatch(gameserver.NewHandlerParameter(c, command.Move{Direction: rules.DirectionSouth})))
	}
	require.NoError(t, gs.dispatch(gameserver.NewHandlerParameter(c, command.Kill{Target: "chanter"})))
	c.sent = nil
	dice.Load([]int{0, 1, 1, 1, 1, 1}) // the imp's pick, then a miss for every swing

	gs.heartbeat(1001, rules.PulseInterval) // a pulse on no other interval

	assert.True(t, swung(c.sent, "imp"), "the imp swung the round it arrived")
}

func swung(sent []any, attacker string) bool {
	for _, m := range sent {
		if s, ok := m.(event.Struck); ok && s.Attacker == attacker {
			return true
		}
	}
	return false
}

// The tick sets the world's pace: ten times the pulse rate, ten times the clock.
func TestSetTickInterval_setsThePace(t *testing.T) {
	gs, _ := newTestGameServer(t)
	gs.SetTickInterval(rules.PulseInterval / 10)
	assert.InDelta(t, 10.0, gs.world.Pace(), 0.001)
}

// Someone already in the start room hears a new character's first arrival as
// that -- a login afterwards is an ordinary one.
func TestCreatePlayer_roomHearsAFirstArrival(t *testing.T) {
	gs, _ := newTestGameServer(t)
	old := &testConn{}
	create(t, gs, old, "oldtimer", "sekrit")
	old.sent = nil

	create(t, gs, &testConn{}, "newbie", "sekrit")

	assert.Contains(t, old.sent, event.EnteredGame{Actor: "Newbie", First: true})
}

// blockingStore's Load waits until the test lets it through: a database
// having a bad few seconds.
type blockingStore struct {
	player.Store
	release chan struct{}
}

func (b *blockingStore) Load(name string) (*player.Record, bool, error) {
	<-b.release
	return b.Store.Load(name)
}

// The world goroutine never waits on the store: a name typed at the door is
// looked up on a goroutine of its own, so a slow database slows that one
// login, not the game.
func TestLogin_theWorldDoesntWaitOnTheStore(t *testing.T) {
	gs, store := newTestGameServer(t)
	slow := &blockingStore{Store: store, release: make(chan struct{})}
	gs.store = slow

	done := make(chan error, 1)
	c := &testConn{}
	go func() { done <- gs.dispatch(gameserver.NewHandlerParameter(c, command.Login{Name: "bob"})) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("the world goroutine waited on the store")
	}

	close(slow.release)
	settle(t, gs)
	assert.Equal(t, []any{event.LoginFailed{Reason: event.NoSuchPlayer}}, c.sent)
}

// A record loaded before the password was checked is stale if the character
// logged out meanwhile -- another session quitting with newer gear. It's
// loaded again, and that's the one that comes in.
func TestLogin_reloadsARecordLoggedOutMeanwhile(t *testing.T) {
	gs, store := newTestGameServer(t)
	first := &testConn{}
	create(t, gs, first, "bob", "sekrit")
	stale := first.Player().Record()
	first.Player().AddCoins(99)
	require.NoError(t, gs.dispatch(gameserver.NewHandlerParameter(first, command.Logout{})))
	fresh, _, err := store.Load("Bob")
	require.NoError(t, err)
	require.Equal(t, 99, fresh.Coins)

	c := &testConn{}
	require.NoError(t, gs.dispatch(gameserver.NewHandlerParameter(c, loginChecked{Name: "Bob", Ok: true, Rec: stale, Gen: 0})))
	for c.Player() == nil {
		settle(t, gs)
	}
	assert.Equal(t, 99, c.Player().Coins(), "the record from after the logout")
}
