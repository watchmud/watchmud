package telnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// what a player types is text by the time anyone else sees it
func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"say hello":                    "say hello",
		"emote \xff\xfb\x01waves":      "emote waves", // a doubled IAC arrives as one 0xFF
		"say \x1b[2Jgone":              "say [2Jgone", // no escape, no clear-screen
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
