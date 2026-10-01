package telnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/watchmud/watchmud/event"
)

func TestPrompt_healthIsColored(t *testing.T) {
	assert.Equal(t, "<"+green+"100/100"+reset+"hp> ", render(event.Prompt{CurrentHealth: 100, MaxHealth: 100}, "testdood"))
	assert.Equal(t, "<"+yellow+"50/100"+reset+"hp> ", render(event.Prompt{CurrentHealth: 50, MaxHealth: 100}, "testdood"))
	assert.Equal(t, "<"+red+"33/100"+reset+"hp> ", render(event.Prompt{CurrentHealth: 33, MaxHealth: 100}, "testdood"))
}

func TestRenderRoom_colored(t *testing.T) {
	got := render(event.RoomDescription{
		Name:    "Temple Square",
		Exits:   "North",
		Objects: []string{"A knife lies here."},
		Mobs:    []string{"A goose honks."},
		Players: []string{"bob"},
	}, "testdood")

	assert.Equal(t, boldCyan+"Temple Square"+reset+"\n"+
		cyan+"[ Exits: North ]"+reset+"\n"+
		green+"A knife lies here."+reset+"\n"+
		yellow+"A goose honks."+reset+"\n"+
		bold+"bob"+reset+" is here.\n", got)
}

// The connection is plain until the world says this player wants color, so
// the login conversation never has any.
func TestFrame_colorOnlyOnceTheWorldSaysSo(t *testing.T) {
	c := loggedInConn()
	died := event.Died{Target: "goose"}

	assert.Equal(t, "goose is dead!\n", c.frame(died))
	assert.Equal(t, "", c.frame(event.Color{On: true}), "the login setting says nothing")
	assert.Equal(t, boldRed+"goose is dead!"+reset+"\n", c.frame(died))
}

func TestFrame_colorOffStripsIt(t *testing.T) {
	c := loggedInConn()
	c.frame(event.Color{On: true})

	assert.Equal(t, "Color is off.\n", c.frame(event.Color{On: false, Changed: true}))
	assert.Equal(t, "goose is dead!\n", c.frame(event.Died{Target: "goose"}))
	assert.Equal(t, "Color is "+boldCyan+"on"+reset+".\n", c.frame(event.Color{On: true, Changed: true}),
		"the answer is in the new setting")
}
