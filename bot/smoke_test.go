package bot_test

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/bot"
	"github.com/watchmud/watchmud/dice"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/memstore"
	"github.com/watchmud/watchmud/server"
	"github.com/watchmud/watchmud/telnet"
	"github.com/watchmud/watchmud/world"
)

// startGame runs the real content -- not testcontent: a content change that
// breaks the walk is exactly what this is for -- on 127.0.0.1:0, at a hundred
// pulses a second.
func startGame(t *testing.T) string {
	t.Helper()
	content, err := loader.LoadContent(os.DirFS("../content"))
	require.NoError(t, err)
	store := memstore.New()
	w, err := world.New(content, store, dice.New([32]byte{}))
	require.NoError(t, err)
	gs := server.New(w, content.Catalog, store)
	gs.SetTickInterval(10 * time.Millisecond)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{}, 2)
	go func() { _ = telnet.Serve(ctx, ln, gs, content.Catalog); done <- struct{}{} }()
	go func() { _ = gs.Run(ctx); done <- struct{}{} }()
	t.Cleanup(func() { cancel(); <-done; <-done })
	return ln.Addr().String()
}

func dial(t *testing.T, addr string) *bot.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := bot.Dial(ctx, addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// createCharacter is the conversation a person has once, by hand, on
// production. It lives here, in a test, so the bot binary can't reach it.
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

func TestSmoke_againstTheRealWorld(t *testing.T) {
	addr := startGame(t)
	createCharacter(t, addr, "Tester", "correcthorse")

	c := dial(t, addr)
	var log strings.Builder
	res, err := bot.Smoke(c, bot.Config{Name: "Tester", Password: "correcthorse"}, &log)
	t.Log("\n" + log.String())
	require.NoError(t, err, "%s\n--- transcript ---\n%s", log.String(), c.Transcript())
	assert.Empty(t, res.Notes, "a fresh world has geese, so the fight must have been tested")
}
