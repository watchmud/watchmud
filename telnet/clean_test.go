package telnet

import (
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
