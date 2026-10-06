package bot

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pipeGame is the server end of a net.Pipe: the test says what the game says
// and hears each line the bot types.
type pipeGame struct {
	t     *testing.T
	nc    net.Conn
	lines chan string
}

var quickPace = Pace{Quiet: 100 * time.Millisecond, Poll: time.Millisecond, Answer: time.Second, Idle: 10 * time.Millisecond}

// adventurer is a quiet bot (no phrases for "Testbot") on a pipe.
func adventurer(t *testing.T) (*Adventurer, *pipeGame) {
	t.Helper()
	server, client := net.Pipe()
	a := NewAdventurer(AdventurerConfig{Name: "Testbot", Password: "x", Siblings: []string{"Pim"}, Seed: 1, Pace: quickPace})
	a.c = newClient(client)
	g := &pipeGame{t: t, nc: server, lines: make(chan string, 64)}
	go func() {
		r := bufio.NewReader(server)
		for {
			l, err := r.ReadString('\n')
			if err != nil {
				close(g.lines)
				return
			}
			g.lines <- strings.TrimRight(l, "\r\n")
		}
	}()
	t.Cleanup(func() { _ = a.c.Close(); _ = server.Close() })
	return a, g
}

func (g *pipeGame) say(s string) {
	g.t.Helper()
	_, err := g.nc.Write([]byte(s))
	require.NoError(g.t, err)
}

func (g *pipeGame) heard() string {
	g.t.Helper()
	select {
	case l := <-g.lines:
		return l
	case <-time.After(2 * time.Second):
		g.t.Fatal("the bot said nothing")
		return ""
	}
}

// One honest answer per person per ten minutes, and never to a fellow bot --
// two bots answering each other would never stop.
func TestTold_oncePerSenderAndNeverASibling(t *testing.T) {
	a, _ := adventurer(t)
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }

	a.told(tell{"Bob", "hi"})
	a.told(tell{"Bob", "hi"})
	a.told(tell{"Pim", "hi"})
	a.told(tell{"testbot", "hi"})
	assert.Equal(t, []tell{{"Bob", "hi"}}, a.pendingTells)

	a.pendingTells = nil
	now = now.Add(9 * time.Minute)
	a.told(tell{"Bob", "hi"})
	assert.Empty(t, a.pendingTells, "still inside ten minutes")
	now = now.Add(2 * time.Minute)
	a.told(tell{"Bob", "hi"})
	assert.Equal(t, []tell{{"Bob", "hi"}}, a.pendingTells)
}

func TestAsk_answersTellsFirst(t *testing.T) {
	a, g := adventurer(t)
	a.pendingTells = []tell{{"Bob", "hi"}}
	done := make(chan error, 1)
	go func() {
		_, _, err := a.ask("look", roomRe("Temple Square"))
		done <- err
	}()

	assert.Equal(t, "tell Bob I'm a bot (see 'help bots') - I can't chat, sorry!", g.heard())
	g.say("Ok.\r\n<100/100hp> ")
	assert.Equal(t, "look", g.heard())
	g.say("Temple Square\r\n A square.\r\n[ Exits: n e s ]\r\n<100/100hp> ")
	require.NoError(t, <-done)
	assert.Equal(t, 1, a.Stats().TellsAnswered)
}

func TestFight_endsQuietAndLoots(t *testing.T) {
	a, g := adventurer(t)
	a.ground = &grounds[0]
	done := make(chan error, 1)
	go func() { done <- a.fight(context.Background()) }()

	g.say("You hit angry goose for 3 damage.\r\nangry goose is dead!\r\n<95/100hp> ")
	assert.Equal(t, "get all from corpse", g.heard(), "after Quiet goes by without a blow")
	g.say("You get a long goose feather from the corpse of angry goose.\r\n<95/100hp> ")
	require.NoError(t, <-done)
	assert.Equal(t, 1, a.Stats().Kills)
	assert.Equal(t, 1, a.Stats().Looted)
	assert.Equal(t, 1, a.carrying)
	assert.False(t, a.attacked)
}

func TestFight_fleesWhenHurt(t *testing.T) {
	a, g := adventurer(t)
	a.ground = &grounds[0]
	a.attacked = true
	done := make(chan error, 1)
	go func() { done <- a.fight(context.Background()) }()

	g.say("wild dog hits you for 60 damage.\r\n<20/100hp> ")
	assert.Equal(t, "flee", g.heard())
	g.say("You flee head over heels.\r\nFarm Lane\r\n A lane.\r\n[ Exits: n s e w ]\r\n<20/100hp> ")
	err := <-done
	assert.ErrorIs(t, err, errLost, "it's somewhere it didn't choose: recall")
}

func TestFight_death(t *testing.T) {
	a, g := adventurer(t)
	a.ground = &grounds[0]
	a.attacked = true
	done := make(chan error, 1)
	go func() { done <- a.fight(context.Background()) }()

	g.say("wild dog hits you for 9 damage.\r\nYou are dead!\r\nTemple Square\r\n A square.\r\n[ Exits: n e s ]\r\n<1/100hp> ")
	require.NoError(t, <-done)
	assert.True(t, a.died)
	assert.Equal(t, 1, a.health)
}

// A real player in a patrol room has the ground to themselves for a while.
func TestHunt_leavesAGroundToAPlayer(t *testing.T) {
	a, g := adventurer(t)
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }
	a.ground = &grounds[0]
	done := make(chan error, 1)
	go func() {
		next, err := a.hunt(context.Background())
		assert.NotNil(t, next)
		done <- err
	}()

	assert.Equal(t, "west", g.heard())
	g.say("The Millpond\r\n A pond.\r\n[ Exits: e s ]\r\nAn angry goose lowers its neck and hisses at you.\r\nPim is here.\r\nBob is here.\r\n<100/100hp> ")
	require.NoError(t, <-done)
	assert.Equal(t, 1, a.Stats().Avoided)
	for range 20 {
		assert.Equal(t, &grounds[1], a.pickGround(1), "only the other ground, while they're there")
	}
	a.avoiding[grounds[1].name] = now.Add(avoidFor)
	assert.Nil(t, a.pickGround(1), "both taken: wait in town")
	now = now.Add(avoidFor + time.Minute)
	assert.NotNil(t, a.pickGround(1))
}

// Resting reads health off prompts it asks for with a bare Enter, since regen
// on its own prints nothing.
func TestRest_pollsUntilRested(t *testing.T) {
	a, g := adventurer(t)
	a.health, a.maxHealth = 40, 100
	done := make(chan error, 1)
	go func() { done <- a.rest(context.Background()) }()

	// it sits down to rest, polls its prompt until it's healed, and gets up
	assert.Equal(t, "rest", g.heard())
	g.say("You sit back and rest.\r\n<40/100hp> ")
	assert.Equal(t, "", g.heard())
	g.say("<60/100hp> ")
	assert.Equal(t, "", g.heard())
	g.say("<95/100hp> ")
	assert.Equal(t, "stand", g.heard())
	g.say("You stand up.\r\n<95/100hp> ")
	require.NoError(t, <-done)
	assert.Equal(t, 95, a.health)
}

// With more than one ground open, bots spread out rather than all taking the
// first in the list.
func TestPickGround_spreadsOut(t *testing.T) {
	a, _ := adventurer(t)
	seen := map[string]int{}
	for range 100 {
		seen[a.pickGround(1).name]++
	}
	for _, g := range grounds {
		assert.Positive(t, seen[g.name], "never chose %s", g.name)
	}
	assert.Nil(t, a.pickGround(99), "nothing suits power 99")
}

// A recall inside the cooldown waits a moment and tries again: dying twice
// within a minute shouldn't leave a bot standing in the death room.
func TestRecall_waitsOutTheCooldown(t *testing.T) {
	retry := recallRetry
	recallRetry = 10 * time.Millisecond
	t.Cleanup(func() { recallRetry = retry })
	a, g := adventurer(t)
	done := make(chan error, 1)
	go func() { done <- a.recall() }()

	assert.Equal(t, "recall", g.heard())
	g.say("You can't recall again yet.\r\n<100/100hp 100/100m> ")
	assert.Equal(t, "recall", g.heard(), "again")
	g.say("Temple Square\r\n A square.\r\n[ Exits: n e s ]\r\n<100/100hp 100/100m> ")

	require.NoError(t, <-done)
	assert.Equal(t, home, a.here)
}
