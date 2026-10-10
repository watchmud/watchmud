package telnet

import (
	"bytes"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// msspPairs reads a subnegotiation back into name -> value.
func msspPairs(t *testing.T, b []byte) map[string]string {
	t.Helper()
	require.True(t, bytes.HasPrefix(b, []byte{IAC, SB, optMSSP}))
	require.True(t, bytes.HasSuffix(b, []byte{IAC, SE}))
	body := string(b[3 : len(b)-2])
	pairs := map[string]string{}
	for _, v := range strings.Split(body, string(rune(msspVar)))[1:] {
		name, value, ok := strings.Cut(v, string(rune(msspVal)))
		require.True(t, ok, "%q has a value", v)
		pairs[name] = value
	}
	return pairs
}

func TestMSSP_whatACrawlerIsTold(t *testing.T) {
	m := &MSSP{Hostname: "watchmud.com", Website: "https://www.watchmud.com", Port: 4000, TLSPort: 4443,
		Players: func() int { return 3 }, Started: time.Unix(1790000000, 0)}
	got := msspPairs(t, m.subnegotiation())
	assert.Equal(t, "WatchMUD", got["NAME"])
	assert.Equal(t, "3", got["PLAYERS"])
	assert.Equal(t, "1790000000", got["UPTIME"])
	assert.Equal(t, "watchmud.com", got["HOSTNAME"])
	assert.Equal(t, "4000", got["PORT"])
	assert.Equal(t, "4443", got["SSL"])
	assert.Equal(t, "1", got["GMCP"])
	assert.NotContains(t, got, "CONTACT", "unsaid when empty")
}

func TestMSSP_noTLSNoSSL(t *testing.T) {
	m := &MSSP{Port: 4000, Hostname: "evil\x01host\xff"}
	got := msspPairs(t, m.subnegotiation())
	assert.NotContains(t, got, "SSL")
	assert.Equal(t, "0", got["PLAYERS"], "nobody counted, nobody playing")
	assert.Equal(t, "evilhost", got["HOSTNAME"], "nothing that would end it early")
}

// Offered after the other options, and answered when a crawler says DO.
func TestMSSP_offeredAndAnswered(t *testing.T) {
	serverEnd, client := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	m := &MSSP{Port: 4000, Players: func() int { return 1 }}
	go start(serverEnd, &fakeServer{}, nil, listener{banner: "Welcome.\r\n", mssp: m}, "test",
		&addressLimit{max: 1, open: map[string]int{}})

	buf := make([]byte, len("Welcome.\r\n")+15)
	_, err := io.ReadFull(client, buf)
	require.NoError(t, err)
	assert.True(t, bytes.HasSuffix(buf, []byte{IAC, WILL, optMSSP}))

	go func() { _, _ = client.Write([]byte{IAC, DO, optMSSP}) }()
	want := m.subnegotiation()
	got := make([]byte, 0, len(want))
	for !bytes.HasSuffix(got, []byte{IAC, SE}) {
		b := make([]byte, 256)
		n, err := client.Read(b)
		require.NoError(t, err)
		got = append(got, b[:n]...)
	}
	assert.True(t, bytes.HasSuffix(got, want), "the answer, after whatever the login conversation said")
}
