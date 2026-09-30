package bot

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listenOnce accepts one connection and hands it to the test.
func listenOnce(t *testing.T) (string, <-chan net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	conns := make(chan net.Conn, 1)
	go func() {
		nc, err := ln.Accept()
		if err != nil {
			return
		}
		t.Cleanup(func() { _ = nc.Close() })
		conns <- nc
	}()
	return ln.Addr().String(), conns
}

func connect(t *testing.T) (*Client, net.Conn) {
	t.Helper()
	addr, conns := listenOnce(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c, err := Dial(ctx, addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c, <-conns
}

func TestExpect_waitsForTextSplitAcrossReads(t *testing.T) {
	c, nc := connect(t)
	go func() {
		_, _ = io.WriteString(nc, "Welc")
		time.Sleep(20 * time.Millisecond)
		_, _ = io.WriteString(nc, "ome to WatchMUD.\r\n")
	}()
	m, err := c.Expect(`Welcome to (\w+)\.\n`, time.Second)
	require.NoError(t, err)
	assert.Equal(t, []string{"Welcome to WatchMUD.\n", "WatchMUD"}, m)
}

// What one Expect matched, the next can't match again: "wait for the second
// goose to die" has to mean the second one.
func TestExpect_consumesWhatItMatched(t *testing.T) {
	c, nc := connect(t)
	_, _ = io.WriteString(nc, "goose is dead!\r\ngoose is dead!\r\n")
	for range 2 {
		_, err := c.Expect(`goose is dead!`, time.Second)
		require.NoError(t, err)
	}
	_, err := c.Expect(`goose is dead!`, 50*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "goose is dead!", "a failure quotes what did arrive")
}

// The server turns echo off around the password prompt; none of that
// negotiation is text.
func TestExpect_ignoresTelnetNegotiation(t *testing.T) {
	c, nc := connect(t)
	_, _ = nc.Write([]byte("Pass\xff\xfb\x01word: \xff\xfa\x18\x01\xff\xff\xff\xf0ok \xff\xffend"))
	m, err := c.Expect(`Password: ok (.)end`, time.Second)
	require.NoError(t, err)
	// a regexp's \xff is the code point U+00FF, not the byte: compare the byte
	assert.Equal(t, "\xff", m[1], "a doubled IAC is a literal 0xFF")
}

func TestExpect_failsWhenTheConnectionCloses(t *testing.T) {
	c, nc := connect(t)
	_, _ = io.WriteString(nc, "Goodbye.\r\n")
	require.NoError(t, nc.Close())
	_, err := c.Expect(`never`, time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

func TestExpectClosed(t *testing.T) {
	c, nc := connect(t)
	assert.Error(t, c.ExpectClosed(50*time.Millisecond), "still open")
	require.NoError(t, nc.Close())
	assert.NoError(t, c.ExpectClosed(time.Second))
}

func TestSend_writesALine(t *testing.T) {
	c, nc := connect(t)
	require.NoError(t, c.Send("look"))
	buf := make([]byte, 6)
	_, err := io.ReadFull(nc, buf)
	require.NoError(t, err)
	assert.Equal(t, "look\r\n", string(buf))
}

// The transcript is printed when a deploy's smoke test fails; the password
// mustn't be in it.
func TestTranscript_hidesSecrets(t *testing.T) {
	c, nc := connect(t)
	_, _ = io.WriteString(nc, "Password: ")
	_, err := c.Expect(`Password: `, time.Second)
	require.NoError(t, err)
	require.NoError(t, c.SendSecret("hunter22"))
	require.NoError(t, c.Send("look"))

	tr := c.Transcript()
	assert.NotContains(t, tr, "hunter22")
	assert.Equal(t, "Password: \n> ******\n> look\n", tr)
}

// deploy.sh runs the bot the moment compose returns, while the server may
// still be loading content.
func TestDial_retriesUntilListening(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	go func() {
		time.Sleep(600 * time.Millisecond)
		late, err := net.Listen("tcp", addr)
		if err != nil {
			return
		}
		t.Cleanup(func() { _ = late.Close() })
		nc, err := late.Accept()
		if err == nil {
			t.Cleanup(func() { _ = nc.Close() })
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := Dial(ctx, addr)
	require.NoError(t, err)
	_ = c.Close()
}

func TestDial_givesUpWhenTheContextDoes(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err = Dial(ctx, addr)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), addr))
}

// A chunk is everything up to a prompt, and the prompt is where the health is.
func TestReadChunk_splitsOnPrompts(t *testing.T) {
	c, nc := connect(t)
	_, _ = io.WriteString(nc, "The Millpond\r\n A pond.\r\n<97/100hp> angry goose hits you for 2 damage.\r\n<95/100hp> ")

	ch, err := c.ReadChunk(time.Second)
	require.NoError(t, err)
	assert.Equal(t, Chunk{Text: "The Millpond\n A pond.\n", Health: 97, MaxHealth: 100}, ch)

	ch, err = c.ReadChunk(time.Second)
	require.NoError(t, err)
	assert.Equal(t, Chunk{Text: "angry goose hits you for 2 damage.\n", Health: 95, MaxHealth: 100}, ch)
}

// Nothing happening is not a failure: a resting bot waits on a quiet world.
func TestReadChunk_quietIsNotAnError(t *testing.T) {
	c, nc := connect(t)
	_, _ = io.WriteString(nc, "half a line with no prompt")
	ch, err := c.ReadChunk(50 * time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, Chunk{}, ch)

	_, _ = io.WriteString(nc, "\r\n<100/100hp> ")
	ch, err = c.ReadChunk(time.Second)
	require.NoError(t, err)
	assert.Equal(t, "half a line with no prompt\n", ch.Text, "the partial text waited for its prompt")
}

func TestReadChunk_closedIsAnError(t *testing.T) {
	c, nc := connect(t)
	require.NoError(t, nc.Close())
	_, err := c.ReadChunk(time.Second)
	assert.Error(t, err)
}
