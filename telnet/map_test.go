package telnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/rules"
)

// A shut door is a #, a way down a v, and the legend says only what's drawn.
func TestRenderMap_doorsAndLevels(t *testing.T) {
	m := event.Map{Zone: "The Drowned Mill", Rooms: []event.MapRoom{
		{X: -1, Y: 0, Name: "The Wheel Pit", Exits: []event.MapExit{{Direction: rules.DirectionEast, Closed: true}}},
		{X: 0, Y: 0, Name: "The Flooded Cellar", Here: true, Exits: []event.MapExit{
			{Direction: rules.DirectionWest, Closed: true}, {Direction: rules.DirectionUp, Leads: "The Mill House"}}},
		{X: 0, Y: -1, Name: "Below", Exits: []event.MapExit{{Direction: rules.DirectionDown}}},
	}}
	assert.Equal(t, "The Drowned Mill\n"+
		" [ ]#[*]\n"+
		"\n"+
		"     [v]\n"+
		"You are at the * (The Flooded Cellar). From here: up to The Mill House.\n"+
		"^ v + a way up, down or both; # a closed door.\n",
		plain(renderMap(m)))
}

func TestRenderMap_nothing(t *testing.T) {
	assert.Equal(t, "You can't make out a map of this place.\n", renderMap(event.Map{}))
}
