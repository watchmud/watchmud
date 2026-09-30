// Package bot is a player that is a program: a telnet client that logs in,
// types commands and reads what comes back, as a person's client would. It
// imports nothing of the server on purpose. It tests the text a player sees,
// and when that text changes, the bot is meant to be what notices.
package bot

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"
)

// dialRetry is how often Dial tries again while nothing is listening yet.
const dialRetry = 250 * time.Millisecond

// tailSize is how much of what arrived a failed Expect quotes.
const tailSize = 2048

// Client is one connection to the game.
type Client struct {
	nc net.Conn

	mu         sync.Mutex
	unread     []byte // received, not yet consumed by an Expect
	transcript strings.Builder
	closed     bool
	// changed is closed, and replaced, whenever unread or closed changes:
	// what an Expect with a deadline waits on.
	changed chan struct{}
}

// Dial connects to addr, trying again until it can or ctx is done.
func Dial(ctx context.Context, addr string) (*Client, error) {
	var d net.Dialer
	for {
		nc, err := d.DialContext(ctx, "tcp", addr)
		if err == nil {
			c := &Client{nc: nc, changed: make(chan struct{})}
			go c.read()
			return c, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("connecting to %s: %w", addr, err)
		case <-time.After(dialRetry):
		}
	}
}

// read is the only reader of the socket. Telnet commands are dropped, and so
// is every \r: the server sends \r\n, and patterns are simpler with \n.
func (c *Client) read() {
	var strip iacStripper
	buf := make([]byte, 4096)
	for {
		n, err := c.nc.Read(buf)
		text := bytes.ReplaceAll(strip.strip(buf[:n]), []byte("\r"), nil)
		c.mu.Lock()
		c.unread = append(c.unread, text...)
		c.transcript.Write(text)
		if err != nil {
			c.closed = true
		}
		close(c.changed)
		c.changed = make(chan struct{})
		c.mu.Unlock()
		if err != nil {
			return
		}
	}
}

// Send types a line.
func (c *Client) Send(line string) error { return c.send(line, line) }

// SendSecret types a line the transcript doesn't show.
func (c *Client) SendSecret(line string) error { return c.send(line, "******") }

func (c *Client) send(line, shown string) error {
	c.mu.Lock()
	// what was typed on a line of its own, even after a prompt
	if s := c.transcript.String(); s != "" && !strings.HasSuffix(s, "\n") {
		c.transcript.WriteString("\n")
	}
	c.transcript.WriteString("> " + shown + "\n")
	c.mu.Unlock()
	if _, err := io.WriteString(c.nc, line+"\r\n"); err != nil {
		return fmt.Errorf("sending %q: %w", shown, err)
	}
	return nil
}

// Expect waits until pattern matches what has arrived since the last match,
// consumes through the end of the match, and returns it and its submatches.
// A pattern is the bot's own code, so a bad one panics.
func (c *Client) Expect(pattern string, timeout time.Duration) ([]string, error) {
	re := regexp.MustCompile(pattern)
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		c.mu.Lock()
		if loc := re.FindSubmatchIndex(c.unread); loc != nil {
			groups := make([]string, len(loc)/2)
			for i := range groups {
				if loc[2*i] >= 0 {
					groups[i] = string(c.unread[loc[2*i]:loc[2*i+1]])
				}
			}
			c.unread = c.unread[loc[1]:]
			c.mu.Unlock()
			return groups, nil
		}
		closed, changed := c.closed, c.changed
		c.mu.Unlock()
		if closed {
			return nil, fmt.Errorf("connection closed while waiting for %q; last received:\n%s", pattern, c.tail())
		}
		select {
		case <-changed:
		case <-deadline.C:
			return nil, fmt.Errorf("nothing matched %q in %s; last received:\n%s", pattern, timeout, c.tail())
		}
	}
}

// ExpectClosed waits for the server to hang up.
func (c *Client) ExpectClosed(timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		c.mu.Lock()
		closed, changed := c.closed, c.changed
		c.mu.Unlock()
		if closed {
			return nil
		}
		select {
		case <-changed:
		case <-deadline.C:
			return fmt.Errorf("still connected after %s; last received:\n%s", timeout, c.tail())
		}
	}
}

// Transcript is everything received, and every line sent marked "> ".
func (c *Client) Transcript() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.transcript.String()
}

func (c *Client) tail() string {
	s := c.Transcript()
	if len(s) > tailSize {
		s = "..." + s[len(s)-tailSize:]
	}
	return s
}

// Close hangs up.
func (c *Client) Close() error {
	return c.nc.Close()
}
