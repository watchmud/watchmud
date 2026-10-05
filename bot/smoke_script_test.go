package bot

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exchange is one line the fake expects and what it answers.
type exchange struct{ want, reply string }

// fakeGame serves one connection: a greeting, then each exchange in turn.
// A line it didn't expect is answered with a complaint the bot won't match,
// so the test fails by timeout with the complaint in the transcript. After
// "quit" it hangs up; after any other last line it waits for the bot to.
func fakeGame(t *testing.T, greeting string, script ...exchange) *Client {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		nc, err := ln.Accept()
		if err != nil {
			return
		}
		defer nc.Close()
		_, _ = io.WriteString(nc, greeting)
		r := bufio.NewReader(nc)
		for _, x := range script {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if got := strings.TrimRight(line, "\r\n"); got != x.want {
				_, _ = fmt.Fprintf(nc, "FAKE: wanted %q, got %q\r\n", x.want, got)
				_, _ = io.Copy(io.Discard, r)
				return
			}
			if x.want == "quit" {
				return
			}
			_, _ = io.WriteString(nc, x.reply)
		}
		_, _ = io.Copy(io.Discard, r)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c, err := Dial(ctx, ln.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// quick shortens the waits for a test that expects one to run out.
func quick(t *testing.T) {
	step, fight := stepTimeout, fightTimeout
	stepTimeout, fightTimeout = 300*time.Millisecond, 300*time.Millisecond
	t.Cleanup(func() { stepTimeout, fightTimeout = step, fight })
}

const greeting = "Welcome to WatchMUD.\r\nBy what name do you wish to be known? "

// fakeRoom renders a room the way the server does, prompt and all.
func fakeRoom(name string, lines ...string) string {
	s := name + "\r\n A place.\r\n[ Exits: n s ]\r\n"
	for _, l := range lines {
		s += l + "\r\n"
	}
	return s + "<100/100hp> "
}

var loggedIn = []exchange{
	{"Tester", "Password: \xff\xfb\x01"},
	{"hunter22", "\xff\xfc\x01\r\n" + fakeRoom("The Millpond")},
	{"recall", fakeRoom("Temple Square")},
	{"south", fakeRoom("Market Square")},
	{"south", fakeRoom("South Gate")},
	{"south", fakeRoom("Outside South Gate")},
	{"south", fakeRoom("southern path")},
	{"south", fakeRoom("The Waystone")},
}

var cfg = Config{Name: "Tester", Password: "hunter22"}

func script(tail ...exchange) []exchange {
	return append(append([]exchange{}, loggedIn...), tail...)
}

func TestSmoke_passes(t *testing.T) {
	c := fakeGame(t, greeting, script(
		exchange{"west", fakeRoom("The Millpond",
			"An angry goose lowers its neck and hisses at you.",
			"An angry goose lowers its neck and hisses at you.")},
		exchange{"kill goose", "You hit an angry goose.\r\nangry goose is dead!\r\n<98/100hp> " +
			"\r\nAn angry goose bites you.\r\nangry goose is dead!\r\n<97/100hp> "},
		exchange{"get all from corpse", "You get 4 coins from the corpse of angry goose.\r\n" +
			"You get a long goose feather from the corpse of angry goose.\r\n<97/100hp> "},
		exchange{"drop all.feather", "Dropped.\r\n<97/100hp> "},
		exchange{"recall", fakeRoom("Temple Square")},
		exchange{"quit", ""},
	)...)

	var log strings.Builder
	res, err := Smoke(c, cfg, &log)
	require.NoError(t, err, c.Transcript())
	assert.Empty(t, res.Notes)
	assert.Contains(t, log.String(), "fight")
	assert.NotContains(t, c.Transcript(), "hunter22")
}

// A goose that left coins and no feather: the coins go in the purse, and
// there's nothing to drop.
func TestSmoke_coinsButNoFeather(t *testing.T) {
	c := fakeGame(t, greeting, script(
		exchange{"west", fakeRoom("The Millpond",
			"An angry goose lowers its neck and hisses at you.")},
		exchange{"kill goose", "You hit an angry goose.\r\nangry goose is dead!\r\n<98/100hp> "},
		exchange{"get all from corpse", "You get 4 coins from the corpse of angry goose.\r\n<98/100hp> "},
		exchange{"recall", fakeRoom("Temple Square")},
		exchange{"quit", ""},
	)...)

	res, err := Smoke(c, cfg, io.Discard)
	require.NoError(t, err, c.Transcript())
	assert.Empty(t, res.Notes)
	assert.NotContains(t, c.Transcript(), "drop all.feather")
}

// Somebody got the geese first: that's the world, not the deploy.
func TestSmoke_noGooseIsANote(t *testing.T) {
	c := fakeGame(t, greeting, script(
		exchange{"west", fakeRoom("The Millpond", "The corpse of angry goose is lying here.")},
		exchange{"recall", fakeRoom("Temple Square")},
		exchange{"quit", ""},
	)...)

	res, err := Smoke(c, cfg, io.Discard)
	require.NoError(t, err, c.Transcript())
	assert.Equal(t, []string{NoGoose}, res.Notes)
}

// Names are permanent: the bot must never answer the creation question.
func TestSmoke_neverCreatesACharacter(t *testing.T) {
	quick(t)
	c := fakeGame(t, greeting,
		exchange{"Nobody", "No one by the name of Nobody. Create them? (yn) "})

	_, err := Smoke(c, Config{Name: "Nobody", Password: "hunter22"}, io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create it by hand")
	assert.NotContains(t, c.Transcript(), "> y")
}

func TestSmoke_wrongPassword(t *testing.T) {
	quick(t)
	c := fakeGame(t, greeting,
		exchange{"Tester", "Password: "},
		exchange{"hunter22", "Wrong password.\r\nPassword: "})

	_, err := Smoke(c, cfg, io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wrong password")
}

func TestSmoke_fightThatNeverEnds(t *testing.T) {
	quick(t)
	c := fakeGame(t, greeting, script(
		exchange{"west", fakeRoom("The Millpond", "An angry goose lowers its neck and hisses at you.")},
		exchange{"kill goose", "You miss an angry goose.\r\n<100/100hp> "},
	)...)

	_, err := Smoke(c, cfg, io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fight")
}

func TestSmoke_botDies(t *testing.T) {
	quick(t)
	c := fakeGame(t, greeting, script(
		exchange{"west", fakeRoom("The Millpond", "An angry goose lowers its neck and hisses at you.")},
		exchange{"kill goose", "An angry goose bites you.\r\nYou are dead!\r\n"},
	)...)

	_, err := Smoke(c, cfg, io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the goose won")
}

// A recall inside the cooldown is tried again, a second later, rather than
// failing the deploy: in production a quick run can reach its final recall
// within a minute of its first.
func TestSmoke_recallWaitsOutTheCooldown(t *testing.T) {
	retry := recallRetry
	recallRetry = 10 * time.Millisecond
	t.Cleanup(func() { recallRetry = retry })
	c := fakeGame(t, greeting, script(
		exchange{"west", fakeRoom("The Millpond")},
		exchange{"recall", "You can't recall again yet.\r\n<100/100hp> "},
		exchange{"recall", fakeRoom("Temple Square")},
		exchange{"quit", ""},
	)...)

	_, err := Smoke(c, cfg, io.Discard)
	require.NoError(t, err, c.Transcript())
}

// A character with no temple token can't get home: that's a broken
// character, said plainly, not a minute of retrying.
func TestSmoke_noToken(t *testing.T) {
	c := fakeGame(t, greeting, loggedIn[0], loggedIn[1],
		exchange{"recall", "Nothing you're wearing lets you recall -- a temple token does; the General Store sells them.\r\n<100/100hp> "},
	)

	_, err := Smoke(c, cfg, io.Discard)
	require.ErrorContains(t, err, "temple token")
}
