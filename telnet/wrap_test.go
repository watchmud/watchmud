package telnet

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/event"
)

func TestWrap(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"no width, no wrapping", "the quick brown fox jumps over", 0, "the quick brown fox jumps over"},
		{"fits", "the quick brown fox\n", 20, "the quick brown fox\n"},
		{"at spaces", "the quick brown fox jumps over the lazy dog\n", 20,
			"the quick brown fox\njumps over the lazy\ndog\n"},
		{"each line on its own", "short\nthe quick brown fox jumps over\n", 20,
			"short\nthe quick brown fox\njumps over\n"},
		{"indent carries on", " the quick brown fox jumps over\n", 20,
			" the quick brown fox\n jumps over\n"},
		{"a word wider than the window gets its own line", "see supercalifragilisticexpialidocious now", 20,
			"see\nsupercalifragilisticexpialidocious\nnow"},
		{"a fitting line keeps its spacing", "  tank    12  (plate)\n", 30, "  tank    12  (plate)\n"},
		{"no trailing newline: a prompt or a question", "<100/100hp> ", 20, "<100/100hp> "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, wrap(tc.in, tc.width))
		})
	}
}

// Color takes no room on screen, so it mustn't count toward the width.
func TestWrap_colorTakesNoRoom(t *testing.T) {
	line := paint(colorTell, "Bob tells you, \"hello there\".") // 29 visible
	assert.Equal(t, line, wrap(line, 29))
	assert.Equal(t, 29, visibleWidth(line))
}

func TestWrapWidth(t *testing.T) {
	assert.Equal(t, 0, wrapWidth(0), "unknown")
	assert.Equal(t, 0, wrapWidth(minWrap-1), "nonsense")
	assert.Equal(t, 80, wrapWidth(80))
	assert.Equal(t, maxWrap, wrapWidth(1000))
}

func TestFrame_wrapsToTheWindow(t *testing.T) {
	c := loggedInConn()
	long := event.Welcome{Text: "Welcome to Wrathrock! Type 'help' to see what you can do."}

	assert.Equal(t, long.Text+"\n", c.frame(long), "nothing said, nothing wrapped")
	require.NoError(t, writesTo(c, windowSize(30)))
	assert.Equal(t, "Welcome to Wrathrock! Type\n'help' to see what you can do.\n", c.frame(long))
}

func writesTo(c *conn, msg any) error {
	c.netConn = &writeConn{}
	return c.write(msg)
}

// The whole way: the client says WILL NAWS and its size, and what it's sent
// after that is wrapped to it.
func TestNAWS_endToEnd(t *testing.T) {
	s := startSession(t, &fakeServer{passwords: map[string]string{"bob": "sekrit99"}})

	s.waitFor("known? ")
	_, err := s.client.Write([]byte{IAC, WILL, optNAWS, IAC, SB, optNAWS, 0, 30, 0, 24, IAC, SE})
	require.NoError(t, err)
	_, err = io.WriteString(s.client, "help\r\n")
	require.NoError(t, err)
	s.waitFor("Type a name to log in as, or a\r\nnew one to create a character.\r\n")
	for _, l := range strings.Split(s.transcript(), "\r\n") {
		// a line with a prompt's GA on it ran on into the reply only here: the
		// client's echo of what was typed isn't in the transcript
		if !strings.Contains(l, string([]byte{IAC})) {
			assert.LessOrEqual(t, visibleWidth(l), 30, "%q", l)
		}
	}
}
