package bot

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fastPace keeps the adventurer's own shape at a hundred times the speed:
// the game ticks every 10ms, a real second.
var fastPace = Pace{
	Think:  [2]time.Duration{0, 2 * time.Millisecond},
	Walk:   [2]time.Duration{0, 2 * time.Millisecond},
	Quiet:  150 * time.Millisecond,
	Poll:   20 * time.Millisecond,
	Answer: 5 * time.Second,
	Idle:   200 * time.Millisecond,
}

// runAdventurer starts one and stops it when the test ends, checking it quit
// cleanly.
func runAdventurer(t *testing.T, addr, name string) *Adventurer {
	t.Helper()
	// two, not five: the zone respawns on the wall clock, every few minutes,
	// and at this speed the fields are cleared in seconds
	a := NewAdventurer(AdventurerConfig{Name: name, Password: "correcthorse", Seed: 7, Pace: fastPace, DonateAfter: 2, Log: t.Logf})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx, addr) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Error("the adventurer didn't stop")
		}
	})
	return a
}

// The whole loop, against the real content: out to the fields, kills, loot,
// and a trip back to the donation room.
func TestAdventurer_huntsLootsAndDonates(t *testing.T) {
	addr := startGame(t, 10*time.Millisecond)
	createCharacter(t, addr, "Wren", "correcthorse")
	a := runAdventurer(t, addr, "Wren")

	ok := assert.Eventually(t, func() bool { return a.Stats().Donations >= 1 }, 60*time.Second, 50*time.Millisecond)
	st := a.Stats()
	require.True(t, ok, "stats: %+v", st)
	assert.Positive(t, st.Kills)
	assert.GreaterOrEqual(t, st.Looted, 2)
}

func TestAdventurer_answersATell(t *testing.T) {
	addr := startGame(t, 10*time.Millisecond)
	createCharacter(t, addr, "Wren", "correcthorse")
	createCharacter(t, addr, "Visitor", "correcthorse")
	runAdventurer(t, addr, "Wren")

	v := loginAs(t, addr, "Visitor", "correcthorse")
	require.Eventually(t, func() bool {
		if v.Send("who") != nil {
			return false
		}
		_, err := v.Expect(`(?m)^Wren `, 200*time.Millisecond)
		return err == nil
	}, 20*time.Second, 50*time.Millisecond, "Wren never came online")
	require.NoError(t, v.Send("tell Wren hello?"))
	_, err := v.Expect(`Wren tells you, "I'm a bot \(see 'help bots'\) - I can't chat, sorry!"`, 30*time.Second)
	require.NoError(t, err, v.Transcript())
}

// A player at the millpond has the Hollowfields: the bot sees them on its
// first patrol step, turns round, and waits it out in town. No pulses: at a
// hundred times speed the geese would kill the visitor in about a second, and
// they'd wake in town before the bot arrived.
func TestAdventurer_leavesAGroundToAPlayer(t *testing.T) {
	addr := startGame(t, time.Hour)
	createCharacter(t, addr, "Wren", "correcthorse")
	createCharacter(t, addr, "Visitor", "correcthorse")

	v := loginAs(t, addr, "Visitor", "correcthorse")
	require.NoError(t, recall(v))
	for _, s := range append(append([]step{}, grounds[0].route...), grounds[0].patrol[0]) {
		require.NoError(t, v.Send(s.dir))
		_, err := room(v, s.room)
		require.NoError(t, err, v.Transcript())
	}

	a := runAdventurer(t, addr, "Wren")
	ok := assert.Eventually(t, func() bool { return a.Stats().Avoided >= 1 }, 30*time.Second, 20*time.Millisecond)
	require.True(t, ok, "stats: %+v", a.Stats())
	time.Sleep(500 * time.Millisecond) // a few idle rounds in town
	assert.Zero(t, a.Stats().Kills, "it never fought on a ground a player was using")
}

// Resting depends on this: regen prints nothing, so a bot reads its health
// from the prompt a bare Enter repeats. Against the real server, not a fake.
func TestBareEnterRepeatsThePrompt(t *testing.T) {
	addr := startGame(t, 10*time.Millisecond)
	createCharacter(t, addr, "Wren", "correcthorse")
	c := loginAs(t, addr, "Wren", "correcthorse")
	_, err := c.ReadChunk(time.Second) // the rest of the room, and its prompt
	require.NoError(t, err)

	for range 3 {
		require.NoError(t, c.Send(""))
		ch, err := c.ReadChunk(time.Second)
		require.NoError(t, err)
		assert.Equal(t, 100, ch.MaxHealth, "a prompt came back: %q", c.Transcript())
	}
}
