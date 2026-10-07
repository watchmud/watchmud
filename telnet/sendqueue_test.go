package telnet

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/watchmud/watchmud/event"
)

// A full queue hangs up -- Send never blocks the world -- and after that
// the room's chatter is dropped quietly: a busy room used to log "send queue
// full" once for every line said in it.
func TestSend_fullQueueHangsUpOnce(t *testing.T) {
	serverEnd, clientEnd := net.Pipe()
	defer clientEnd.Close()
	c := newConn(serverEnd, nil, nil) // no pumps: nothing drains the queue

	for range cap(c.sendQueue) {
		assert.NoError(t, c.send(event.Said{Speaker: "bob", Value: "hi"}))
	}
	assert.ErrorContains(t, c.send(event.Said{Speaker: "bob", Value: "hi"}), "send queue full")
	select {
	case <-c.quit:
	default:
		t.Fatal("still open")
	}
	assert.ErrorContains(t, c.send(event.Said{Speaker: "bob", Value: "hi"}), "connection closed")
}
