package telnet

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/event"
)

// queued is everything waiting in c's send queue, in order.
func queued(c *conn) []any {
	var got []any
	for {
		select {
		case m := <-c.sendQueue:
			got = append(got, m)
		default:
			return got
		}
	}
}

func ttypeConn(t *testing.T) *conn {
	serverEnd, client := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = serverEnd.Close() })
	return newConn(serverEnd, &fakeServer{}, nil)
}

var askTType = negotiation([]byte{IAC, SB, optTTYPE, ttypeSend, IAC, SE})

// A client that says WILL TTYPE is asked, and asked again, until MTTS; bit
// 64 is a screen reader.
func TestTerminalType_MTTSWithAScreenReader(t *testing.T) {
	c := ttypeConn(t)
	c.negotiated(WILL, optTTYPE)
	assert.Equal(t, []any{askTType}, queued(c))

	c.subnegotiated(optTTYPE, append([]byte{ttypeIs}, "MUDLET"...))
	assert.Equal(t, []any{askTType}, queued(c))
	c.subnegotiated(optTTYPE, append([]byte{ttypeIs}, "ANSI-TRUECOLOR"...))
	assert.Equal(t, []any{askTType}, queued(c))
	c.subnegotiated(optTTYPE, append([]byte{ttypeIs}, "MTTS 77"...)) // 64+8+4+1
	assert.Equal(t, []any{screenReaderClient{}}, queued(c), "and no more asking")
}

func TestTerminalType_MTTSWithout(t *testing.T) {
	c := ttypeConn(t)
	c.subnegotiated(optTTYPE, append([]byte{ttypeIs}, "MTTS 13"...))
	assert.Empty(t, queued(c))
}

// A client that isn't MTTS repeats its one answer; that ends it.
func TestTerminalType_stopsWhenTheAnswerRepeats(t *testing.T) {
	c := ttypeConn(t)
	c.subnegotiated(optTTYPE, append([]byte{ttypeIs}, "XTERM"...))
	assert.Equal(t, []any{askTType}, queued(c))
	c.subnegotiated(optTTYPE, append([]byte{ttypeIs}, "XTERM"...))
	assert.Empty(t, queued(c))
}

// ... or asking three times.
func TestTerminalType_atMostThreeTimes(t *testing.T) {
	c := ttypeConn(t)
	for _, a := range []string{"A", "B"} {
		c.subnegotiated(optTTYPE, append([]byte{ttypeIs}, a...))
		require.Equal(t, []any{askTType}, queued(c))
	}
	c.subnegotiated(optTTYPE, append([]byte{ttypeIs}, "C"...))
	assert.Empty(t, queued(c))
}

// The client saying so turns the mode on, and the player is told how to
// turn it off; the player's own word, either way, outranks the client.
func TestScreenReader_clientThenPlayer(t *testing.T) {
	c := loggedInConn()
	got := writes(t, c, screenReaderClient{}, event.ScreenReader{On: false}, fullHealth)
	assert.Contains(t, got, "'screenreader off' to stop.\r\n")
	assert.Contains(t, got, "health 100 of 100. "+ga)

	got = writes(t, c, inputReceived{}, event.ScreenReader{On: false, Changed: true}, fullHealth)
	assert.Equal(t, "Screen-reader mode is off.\r\n<100/100hp> "+ga, got)
}

// Saved on, with a client that doesn't say: on, and nothing to explain.
func TestScreenReader_savedOn(t *testing.T) {
	c := loggedInConn()
	got := writes(t, c, event.ScreenReader{On: true}, fullHealth)
	assert.Equal(t, "health 100 of 100. "+ga, got)
}

// Answers nobody asked for are dropped once three are in, not kept.
func TestTerminalType_unaskedAnswersIgnored(t *testing.T) {
	c := ttypeConn(t)
	for i := range 100 {
		c.subnegotiated(optTTYPE, append([]byte{ttypeIs}, byte('A'+i%26), byte('a'+i/26)))
	}
	assert.Len(t, c.ttypes, maxTTypes)
}
