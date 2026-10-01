package telnet

import (
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/player"
)

// frames runs messages through frame in order, as writePump would, and
// returns everything that would have been written.
func frames(c *conn, msgs ...any) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(c.frame(m))
	}
	return b.String()
}

func loggedInConn() *conn {
	c := &conn{}
	c.SetPlayer(player.NewTestPlayer(uuid.New(), "testdood", nil))
	return c
}

var fullHealth = event.Prompt{CurrentHealth: 100, MaxHealth: 100}

func TestRenderPrompt(t *testing.T) {
	assert.Equal(t, "<1/100hp> ", plain(render(event.Prompt{CurrentHealth: 1, MaxHealth: 100}, "testdood")))
}

func TestPrompt_afterACommand(t *testing.T) {
	c := loggedInConn()

	got := frames(c,
		fullHealth, // login
		inputReceived{},
		event.Pong{Target: "testdood"},
		fullHealth,
	)

	assert.Equal(t, "<100/100hp> "+render(event.Pong{Target: "testdood"}, "testdood")+"<100/100hp> ", got)
}

// The server prompts everyone after every command and pulse; only the first
// of a run with nothing between them is printed.
func TestPrompt_onlyWhenSomethingWasSaid(t *testing.T) {
	c := loggedInConn()

	assert.Equal(t, "<100/100hp> ", frames(c, fullHealth, fullHealth, fullHealth))
}

// Answering the prompt with nothing repeats the last one the server sent,
// since the world never hears about a bare Enter.
func TestPrompt_bareEnterRepeatsTheLastPrompt(t *testing.T) {
	c := loggedInConn()
	hurt := event.Prompt{CurrentHealth: 12, MaxHealth: 100}

	assert.Equal(t, "<12/100hp> <12/100hp> ", frames(c, hurt, inputReceived{}, reprompt{}))
}

// Nothing to repeat yet, nothing written.
func TestPrompt_repromptBeforeAnyPrompt(t *testing.T) {
	c := loggedInConn()

	assert.Equal(t, "", frames(c, inputReceived{}, reprompt{}))
}

// Something said while the prompt sits unanswered starts its own line, then
// gets a prompt of its own -- showing what the blow cost.
func TestPrompt_unpromptedOutputStartsANewLine(t *testing.T) {
	c := loggedInConn()

	got := frames(c, fullHealth, "The drone hits you.\n", event.Prompt{CurrentHealth: 97, MaxHealth: 100})

	assert.Equal(t, "<100/100hp> \nThe drone hits you.\n<97/100hp> ", got)
}

// Before login the connection asks its own questions.
func TestPrompt_notBeforeLogin(t *testing.T) {
	c := &conn{}

	assert.Equal(t, "", frames(c, fullHealth))
}
