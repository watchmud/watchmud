package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/watchmud/watchmud/bot"
)

// a name in both lists would be two sessions of one character, and the
// second is refused for as long as the first plays
func TestDuplicate(t *testing.T) {
	assert.Equal(t, "", duplicate([]string{"Wren", "Pim", "Odo"}))
	assert.Equal(t, "odo", duplicate([]string{"Wren", "Odo", "odo"}))
}

func TestStyleOf(t *testing.T) {
	// two hunters, a wanderer, a socialite, two explorers
	want := []bot.Style{bot.Hunter, bot.Hunter, bot.Wanderer, bot.Socialite, bot.Explorer, bot.Explorer}
	for i, s := range want {
		assert.Equal(t, s, styleOf(i, 2, 1, 1), "bot %d", i)
	}
}
