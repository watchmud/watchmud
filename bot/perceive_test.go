package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTellers(t *testing.T) {
	text := "Bob tells you, \"hi\".\nangry goose hits you for 2 damage.\nAnn tells you, \"hello\".\n"
	assert.Equal(t, []tell{{"Bob", "hi"}, {"Ann", "hello"}}, tellers(text))
	assert.Empty(t, tellers("You say, \"Bob tells you, hi\".\n"), "only at the start of a line")
}

func TestAttackedAndBlows(t *testing.T) {
	assert.True(t, attacked("angry goose hits you for 2 damage.\n"))
	assert.True(t, attacked("field rat misses you.\n"))
	assert.False(t, attacked("You hit field rat for 3 damage.\n"))
	assert.False(t, attacked("Pim hits field rat.\n"), "someone else's fight")

	assert.True(t, blows("You miss field rat.\n"))
	assert.True(t, blows("field rat misses you.\n"))
	assert.False(t, blows("Pim hits field rat.\n"))
}

func TestDeaths(t *testing.T) {
	assert.Equal(t, []string{"angry goose", "field rat"}, deaths("angry goose is dead!\nYou hit x.\nfield rat is dead!\n"))
}

// Fellow bots and yourself aren't players to make way for.
func TestPlayersHere(t *testing.T) {
	room := "The Millpond\n A pond.\n[ Exits: e s ]\nAn angry goose lowers its neck and hisses at you.\nWren is here.\nPim is here.\nBob is here.\n"
	assert.Equal(t, []string{"Bob"}, playersHere(room, "wren", []string{"Pim", "Odo"}))
	assert.Empty(t, playersHere("The corpse of field rat is lying here.\n", "Wren", nil))
}

func TestLooted(t *testing.T) {
	assert.Equal(t, 2, looted("You get a scrap of rat pelt from the corpse of field rat.\nYou get a knotted cudgel from the corpse of bandit.\n"))
	assert.Equal(t, 0, looted("There's nothing in there.\n"))
}

func TestPowerAndConsider(t *testing.T) {
	m := powerRe.FindStringSubmatch("You are using (power 3):\nwield\ta dagger\n")
	assert.Equal(t, "3", m[1])
	m = considerRe.FindStringSubmatch("Giant beetle looks like a fair fight. (power 2; you are 1)\n")
	assert.Equal(t, []string{"(power 2; you are 1)", "2", "1"}, m)
}

// Coins go in the purse; they aren't things to take to the donation room.
func TestLooted_coinsDontCount(t *testing.T) {
	text := "You get 7 coins from the corpse of angry goose.\nYou get a long goose feather from the corpse of angry goose.\n"
	assert.Equal(t, 1, looted(text))
}
