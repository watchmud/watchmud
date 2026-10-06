package telnet

import (
	"testing"
	"time"

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

// who: names bold (a bot's dim), the role colored, where they are in cyan;
// padding is counted without the color, so the columns still line up
func TestRenderWho_colored(t *testing.T) {
	got := render(event.Who{Players: []event.WhoEntry{
		{PlayerName: "Al", Lineage: "Human", Role: "Tank", RoomName: "Temple Square", ZoneName: "Wrathrock"},
		{PlayerName: "Wren", Bot: true, Lineage: "Human", RoomName: "The Millpond", ZoneName: "Hollowfields"},
	}}, "Al")

	assert.Equal(t, bold+"Players"+reset+"\n"+
		"  "+bold+"Al"+reset+"    Human "+magenta+"Tank"+reset+"  "+cyan+"Temple Square, Wrathrock"+reset+"\n"+
		"\n"+bold+"Bots"+reset+"\n"+
		"  "+dim+"Wren"+reset+"  Human       "+cyan+"The Millpond, Hollowfields"+reset+"\n"+
		"\n1 player and 1 bot online.\n", got)
}

// abilities: the name colored, ready in green and waiting in yellow, and the
// columns padded by what shows, not by the color codes
func TestRenderAbilities_colored(t *testing.T) {
	got := render(event.Abilities{Granted: []event.GrantedAbility{
		{Name: "heal", Mana: 20, Cooldown: 10 * time.Second, Item: "a censer", Power: 1},
		{Name: "smite", Mana: 5, Cooldown: 20 * time.Second, Item: "a knotted cudgel", Power: 3, ReadyIn: 11500 * time.Millisecond},
	}}, "testdood")

	assert.Equal(t, bold+"Abilities"+reset+"\n"+
		"  "+boldCyan+"heal"+reset+"   20 mana  10s cooldown  a censer (power 1)          "+green+"ready"+reset+"\n"+
		"  "+boldCyan+"smite"+reset+"   5 mana  20s cooldown  a knotted cudgel (power 3)  "+yellow+"ready in 12s"+reset+"\n", got)
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

// mana follows health, uncolored = health is the warning, mana is a number
// a prompt with no mana pool says nothing about mana.
func TestPrompt_mana(t *testing.T) {
	assert.Equal(t, "<"+green+"100/100"+reset+"hp 80/100m> ",
		render(event.Prompt{CurrentHealth: 100, MaxHealth: 100, CurrentMana: 80, MaxMana: 100}, "testdood"))
	// Review Focus 5: empty is still shown
	assert.Equal(t, "<100/100hp 0/100m> ",
		plain(render(event.Prompt{CurrentHealth: 100, MaxHealth: 100, CurrentMana: 0, MaxMana: 100}, "testdood")))
	assert.Equal(t, "<100/100hp> ",
		plain(render(event.Prompt{CurrentHealth: 100, MaxHealth: 100}, "testdood")))
}
