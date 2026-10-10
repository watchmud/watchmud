package telnet

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/event"
)

// Once the client says DO, the start marker goes out plain and everything
// after it is one zlib stream, each write readable as soon as it lands.
func TestMCCP_compressesAfterTheMarker(t *testing.T) {
	c := loggedInConn()
	wc := &writeConn{}
	c.netConn = wc

	require.NoError(t, c.write(compress(true)))
	marker := []byte{IAC, SB, optMCCP2, IAC, SE}
	require.Equal(t, marker, wc.out.Bytes())

	require.NoError(t, c.write(event.Pong{Target: "testdood"}))
	require.NoError(t, c.write(fullHealth))
	compressed := wc.out.Bytes()[len(marker):]

	z, err := zlib.NewReader(bytes.NewReader(compressed))
	require.NoError(t, err)
	want := "Pong testdood.\r\n<100/100hp> " + ga
	got := make([]byte, len(want))
	_, err = io.ReadFull(z, got)
	require.NoError(t, err, "flushed: readable without the stream ending")
	assert.Equal(t, want, string(got))
}

// DONT ends the stream; what follows is plain again.
func TestMCCP_endsOnDont(t *testing.T) {
	c := loggedInConn()
	wc := &writeConn{}
	c.netConn = wc
	require.NoError(t, c.write(compress(true)))
	require.NoError(t, c.write(compress(true)), "twice is once")
	require.NoError(t, c.write(event.Pong{Target: "testdood"}))
	require.NoError(t, c.write(compress(false)))
	before := wc.out.Len()
	require.NoError(t, c.write(inputReceived{}))
	require.NoError(t, c.write(event.Pong{Target: "testdood"}))
	assert.Equal(t, "Pong testdood.\r\n", wc.out.String()[before:])

	z, err := zlib.NewReader(bytes.NewReader(wc.out.Bytes()[5:before]))
	require.NoError(t, err)
	all, err := io.ReadAll(z)
	require.NoError(t, err, "a complete stream")
	assert.Equal(t, "Pong testdood.\r\n", string(all))
}

func TestMCCP_negotiated(t *testing.T) {
	c := ttypeConn(t)
	c.negotiated(DO, optMCCP2)
	c.negotiated(DONT, optMCCP2)
	assert.Equal(t, []any{compress(true), compress(false)}, queued(c))
}

// The whole way round on a socket: a client that says DO MCCP2 at the name
// prompt reads the rest of the login conversation through zlib.
func TestMCCP_overASocket(t *testing.T) {
	serverEnd, client := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	c := newConn(serverEnd, &fakeServer{passwords: map[string]string{"bob": "sekrit99"}}, nil)
	go c.writePump()
	go c.readPump()

	r := bufio.NewReader(client)
	marker := string([]byte{IAC, SB, optMCCP2, IAC, SE})
	var plainPart strings.Builder
	for !strings.HasSuffix(plainPart.String(), "known? "+ga) {
		b, err := r.ReadByte()
		require.NoError(t, err)
		plainPart.WriteByte(b)
	}
	go func() { _, _ = client.Write(append([]byte{IAC, DO, optMCCP2}, "bob\r\n"...)) }()
	for !strings.HasSuffix(plainPart.String(), marker) {
		b, err := r.ReadByte()
		require.NoError(t, err)
		plainPart.WriteByte(b)
	}
	z, err := zlib.NewReader(r)
	require.NoError(t, err)
	want := echoOff + "Password: " + ga
	got := make([]byte, len(want))
	_, err = io.ReadFull(z, got)
	require.NoError(t, err)
	assert.Equal(t, want, string(got))
}
