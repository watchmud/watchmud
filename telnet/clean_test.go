package telnet

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// what a player types is text by the time anyone else sees it
func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"say hello":                    "say hello",
		"emote \xff\xfb\x01waves":      "emote waves", // a doubled IAC arrives as one 0xFF
		"say \x1b[2Jgone":              "say [2Jgone", // no escape, no clear-screen
		"say abc\u202edef":             "say abcdef",  // no right-to-left override
		"say a\tb":                     "say a b",
		"say café, naïve, 日本":          "say café, naïve, 日本",
		"say � is a real char":         "say � is a real char",
		"say bell\x07 and \x00nul\x7f": "say bell and nul",
	} {
		assert.Equal(t, want, clean(in), "%q", in)
	}
}

// and a stray 0xFF that got into text anyway goes out as a byte, not a command
func TestWrite_doublesIAC(t *testing.T) {
	c := loggedInConn()
	got := writes(t, c, "a\xffb\n")
	assert.Equal(t, "a\xff\xffb\r\n", got)
}

// an IPv6 address counts by its /64; IPv4 by itself
func TestHostKey(t *testing.T) {
	assert.Equal(t, "203.0.113.7", hostKey("203.0.113.7"))
	assert.Equal(t, "2001:db8:1:2::/64", hostKey("2001:db8:1:2:aaaa:bbbb:cccc:dddd"))
	assert.Equal(t, hostKey("2001:db8:1:2::1"), hostKey("2001:db8:1:2::ffff"))
	assert.NotEqual(t, hostKey("2001:db8:1:2::1"), hostKey("2001:db8:1:3::1"))
}

// every connection together has a ceiling, whoever they're from
func TestAddressLimit_full(t *testing.T) {
	l := &addressLimit{max: maxConns + 1, open: map[string]int{}}
	for range maxConns {
		ok, _ := l.acquire("a")
		require.True(t, ok)
	}
	ok, full := l.acquire("b")
	assert.False(t, ok)
	assert.True(t, full)
	l.release("a")
	ok, _ = l.acquire("b")
	assert.True(t, ok)
}

// an address makes a household's worth of characters a day, not a script's
func TestAddressLimit_creations(t *testing.T) {
	l := &addressLimit{max: maxConnsPerAddress, open: map[string]int{}}
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	for range createsPerWindow {
		require.True(t, l.mayCreate("a", now))
		l.created("a", now)
	}
	assert.False(t, l.mayCreate("a", now))
	assert.True(t, l.mayCreate("b", now), "another address is its own")
	assert.True(t, l.mayCreate("a", now.Add(createWindow)), "a day on")
	assert.Empty(t, l.made, "and the old ones are forgotten")
}

// kill takes the whole name: "kill wild dog" mustn't be "kill wild", which
// is whichever wild thing is listed first
func TestParse_killTakesTheWholeName(t *testing.T) {
	for _, line := range []string{"kill wild dog", "hit wild dog", "attack wild dog"} {
		cmd, err := parseCommand(strings.Fields(line))
		require.NoError(t, err)
		assert.Equal(t, command.Kill{Target: "wild dog"}, cmd, line)
	}
}

// 'hello and :waves need no space
func TestParse_sayAndEmoteShortcuts(t *testing.T) {
	cmd, err := parseCommand(strings.Fields("'hello there"))
	require.NoError(t, err)
	assert.Equal(t, command.Say{Value: "hello there"}, cmd)
	cmd, err = parseCommand(strings.Fields(":waves"))
	require.NoError(t, err)
	assert.Equal(t, command.Emote{Text: "waves"}, cmd)
}

// "commands" is the world's bare list, not help again
func TestHelp_commandsIsntHelp(t *testing.T) {
	_, ok := helpFor("commands")
	assert.False(t, ok)
	cmd, err := parseCommand([]string{"commands"})
	require.NoError(t, err)
	assert.Equal(t, command.Commands{}, cmd)
}

// one coin is a coin
func TestRender_goldGivenOne(t *testing.T) {
	assert.Equal(t, "A coin appears in your purse. You have 5.\n", plain(render(event.GoldGiven{Amount: 1, Coins: 5}, "ann")))
}

// a table lines up whatever the letters: columns, not bytes
func TestTable_multibyte(t *testing.T) {
	got := table([][]cell{{{text: "café"}, {text: "x"}}, {{text: "cafe"}, {text: "y"}}})
	assert.Equal(t, "  café  x\n  cafe  y\n", got)
}
