package telnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/watchmud/watchmud/event"
)

func TestRenderDied(t *testing.T) {
	died := event.Died{Target: "testdood", IsPlayer: true}

	assert.Equal(t, "You are dead!\n", plain(render(died, "testdood")))
	assert.Equal(t, "testdood is dead!\n", plain(render(died, "otherdood")))
}

func TestRenderEnteredGame(t *testing.T) {
	assert.Equal(t, "testdood has entered the game.\n", render(event.EnteredGame{Actor: "testdood"}, "otherdood"))
	assert.Equal(t, "testdood has entered the game for the first time.\n",
		render(event.EnteredGame{Actor: "testdood", First: true}, "otherdood"))
	assert.Equal(t, "Head south.\n", render(event.Welcome{Text: "Head south."}, "testdood"))
}

// A mob that happens to share your name is not you.
func TestRenderDied_mob(t *testing.T) {
	assert.Equal(t, "testdood is dead!\n", plain(render(event.Died{Target: "testdood"}, "testdood")))
}
