package bot

import (
	"context"
	"net"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/dice"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/memstore"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/server"
	"github.com/watchmud/watchmud/telnet"
	"github.com/watchmud/watchmud/world"
	"golang.org/x/crypto/bcrypt"
)

// startGame runs the real content -- not testcontent: a content change that
// breaks a bot is exactly what these tests are for -- on 127.0.0.1:0,
// ticking every tick. 10ms is a hundred times the real pace; an hour is a
// world where no pulse ever comes, so nothing wanders or picks a fight.
func startGame(t *testing.T, tick time.Duration) string {
	t.Helper()
	addr, _ := startGameStore(t, tick)
	return addr
}

// game is a running game's store and server, for a test that changes a
// character's record between their sessions (setRecord).
type game struct {
	store *memstore.Store
	gs    *server.GameServer
}

// startGameStore is startGame, handing back the game too.
func startGameStore(t *testing.T, tick time.Duration) (string, *game) {
	t.Helper()
	content, err := loader.LoadContent(os.DirFS("../content"))
	require.NoError(t, err)
	store := memstore.New()
	w, err := world.New(content, store, dice.New([32]byte{}))
	require.NoError(t, err)
	w.SetMoonClock(func() time.Time { return world.NewMoon }) // no full moons in a test
	gs := server.New(w, content.Catalog, store)
	gs.SetTickInterval(tick)
	gs.SetPasswordCost(bcrypt.MinCost) // throwaway characters

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{}, 2)
	go func() { _ = telnet.Serve(ctx, ln, gs, content.Catalog); done <- struct{}{} }()
	go func() { _ = gs.Run(ctx); done <- struct{}{} }()
	t.Cleanup(func() { cancel(); <-done; <-done })
	return ln.Addr().String(), &game{store, gs}
}

// setRecord changes a logged-out character's record, with nobody playing.
// Quitting closes the connection before the logout's save -- the server
// notices the hang-up and logs them out -- so it waits for the world to
// count nobody in it, which it does in the prompt after that save. An atomic,
// so the save is seen to have happened.
func setRecord(t *testing.T, g *game, name string, change func(*player.Record)) {
	t.Helper()
	require.Eventually(t, func() bool { return g.gs.Playing() == 0 }, 5*time.Second, 5*time.Millisecond,
		"still in the world")
	rec, ok, err := g.store.Load(name)
	require.NoError(t, err)
	require.True(t, ok, "no record for %s", name)
	r := *rec
	r.Inventory = slices.Clone(rec.Inventory)
	change(&r)
	require.NoError(t, g.store.Save(&r))
}

func dial(t *testing.T, addr string) *Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := Dial(ctx, addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// createCharacter is the conversation a person has once, by hand, on
// production. It lives in a test, so no binary can reach it.
func createCharacter(t *testing.T, addr, name, password string) {
	t.Helper()
	c := dial(t, addr)
	steps := []struct{ want, send string }{
		{`By what name do you wish to be known\? `, name},
		{`Create them\? \(yn\) `, "y"},
		{`Which lineage\? `, "1"},
		{`Choose a password: `, password},
		{`Again: `, password},
		{`\[ Exits: `, "quit"},
	}
	for _, s := range steps {
		_, err := c.Expect(s.want, 10*time.Second)
		require.NoError(t, err, c.Transcript())
		require.NoError(t, c.Send(s.send))
	}
	require.NoError(t, c.ExpectClosed(10*time.Second), c.Transcript())
}

// loginAs is a player at the keyboard, for a test that needs one in the world.
func loginAs(t *testing.T, addr, name, password string) *Client {
	t.Helper()
	c := dial(t, addr)
	require.NoError(t, login(c, Config{Name: name, Password: password}), c.Transcript())
	return c
}
