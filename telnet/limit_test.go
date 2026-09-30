package telnet

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

// dial connects and returns the first line the server says.
func dial(t *testing.T, addr string) (net.Conn, string) {
	t.Helper()
	nc, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = nc.Close() })
	require.NoError(t, nc.SetReadDeadline(time.Now().Add(2*time.Second)))
	line, err := bufio.NewReader(nc).ReadString('\n')
	require.NoError(t, err)
	return nc, strings.TrimSpace(line)
}

// One address gets a few connections -- a household, a second window -- and
// no more, so one client can't eat every slot.
func TestServe_capsConnectionsPerAddress(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	l := listener{ln: ln, banner: "Welcome to WatchMUD.\r\n"}
	go func() { _ = serve(ctx, l, &fakeServer{passwords: map[string]string{}}, nil, newLimit(2)) }()
	addr := ln.Addr().String()

	first, greeting := dial(t, addr)
	assert.Equal(t, "Welcome to WatchMUD.", greeting)
	_, greeting = dial(t, addr)
	assert.Equal(t, "Welcome to WatchMUD.", greeting)

	refused, greeting := dial(t, addr)
	assert.Equal(t, "Too many connections from your address. Try again later.", greeting)
	_, err = bufio.NewReader(refused).ReadString('\n')
	assert.Error(t, err, "and then hung up on")

	// a slot comes back when a connection ends
	require.NoError(t, first.Close())
	require.Eventually(t, func() bool {
		nc, err := net.Dial("tcp", addr)
		if err != nil {
			return false
		}
		defer nc.Close()
		_ = nc.SetReadDeadline(time.Now().Add(time.Second))
		line, _ := bufio.NewReader(nc).ReadString('\n')
		return strings.HasPrefix(line, "Welcome")
	}, 2*time.Second, 20*time.Millisecond)
}

// The plain port says the encrypted one exists; players can't use what they
// don't know about.
func TestBanner(t *testing.T) {
	assert.Equal(t, "Welcome to WatchMUD.\r\n", plainBanner(0))
	assert.Equal(t, "Welcome to WatchMUD.\r\nFor an encrypted connection, use port 4443 with TLS.\r\n", plainBanner(4443))
}

// Serve is the plain port on a listener the caller opened -- a test's
// 127.0.0.1:0 -- greeting the way Listen's does.
func TestServe_onACallersListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = Serve(ctx, ln, &fakeServer{passwords: map[string]string{}}, nil) }()

	_, greeting := dial(t, ln.Addr().String())
	assert.Equal(t, "Welcome to WatchMUD.", greeting)
}
