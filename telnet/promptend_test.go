package telnet

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/event"
)

// writeConn is a net.Conn that keeps what write writes, so a test can see
// the bytes without a pipe and a reader to drain it.
type writeConn struct {
	net.Conn
	out bytes.Buffer
}

func (w *writeConn) Write(p []byte) (int, error)      { return w.out.Write(p) }
func (w *writeConn) SetWriteDeadline(time.Time) error { return nil }

func writes(t *testing.T, c *conn, msgs ...any) string {
	t.Helper()
	wc := &writeConn{}
	c.netConn = wc
	for _, m := range msgs {
		require.NoError(t, c.write(m))
	}
	return wc.out.String()
}

func TestPromptEnd_inGamePromptEndsWithGA(t *testing.T) {
	c := loggedInConn()

	assert.Equal(t, "<100/100hp> "+ga, writes(t, c, fullHealth))
}

// Only a prompt is marked: what the world says is ordinary lines.
func TestPromptEnd_onlyPrompts(t *testing.T) {
	c := loggedInConn()

	got := writes(t, c, fullHealth, inputReceived{}, event.Pong{Target: "testdood"}, fullHealth)

	assert.Equal(t, "<100/100hp> "+ga+"Pong testdood.\r\n<100/100hp> "+ga, got)
}

func TestPromptEnd_repromptIsMarkedToo(t *testing.T) {
	c := loggedInConn()

	got := writes(t, c, fullHealth, inputReceived{}, reprompt{})

	assert.Equal(t, "<100/100hp> "+ga+"<100/100hp> "+ga, got)
}

// A prompt the connection decides not to print isn't marked either: a bare
// GA would tell the client a prompt arrived that never did.
func TestPromptEnd_noMarkWithoutAPrompt(t *testing.T) {
	c := loggedInConn()

	assert.Equal(t, "<100/100hp> "+ga, writes(t, c, fullHealth, fullHealth))
}

func TestPromptEnd_EORonceTheClientAgrees(t *testing.T) {
	c := loggedInConn()

	got := writes(t, c, endOfRecord(true), fullHealth, inputReceived{}, endOfRecord(false), fullHealth)

	assert.Equal(t, "<100/100hp> "+eor+"<100/100hp> "+ga, got)
}

func TestPromptEnd_loginQuestionsAreMarked(t *testing.T) {
	s := startSession(t, &fakeServer{passwords: map[string]string{"bob": "sekrit99"}})

	s.waitFor("known? " + ga)
	_, err := io.WriteString(s.client, "bob\r\n")
	require.NoError(t, err)
	s.waitFor(echoOff + "Password: " + ga)
}

func TestPromptEnd_clientSaysDoEOR(t *testing.T) {
	s := startSession(t, &fakeServer{passwords: map[string]string{"bob": "sekrit99"}})

	s.waitFor("known? ")
	_, err := s.client.Write([]byte{IAC, DO, optEOR})
	require.NoError(t, err)
	_, err = io.WriteString(s.client, "bob\r\n")
	require.NoError(t, err)
	s.waitFor("Password: " + eor)
}

// The offer follows the banner, so a MUD client can answer it before the
// first question.
func TestStart_offersEOR(t *testing.T) {
	serverEnd, client := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	go start(serverEnd, &fakeServer{}, nil, "Welcome.\r\n", "test", &addressLimit{max: 1, open: map[string]int{}})

	buf := make([]byte, len("Welcome.\r\n")+3)
	_, err := io.ReadFull(client, buf)
	require.NoError(t, err)
	assert.Equal(t, "Welcome.\r\n"+string([]byte{IAC, WILL, optEOR}), string(buf))
}
