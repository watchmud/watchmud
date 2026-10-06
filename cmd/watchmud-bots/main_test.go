package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// a name in both lists would be two sessions of one character, and the
// second is refused for as long as the first plays
func TestDuplicate(t *testing.T) {
	assert.Equal(t, "", duplicate([]string{"Wren", "Pim", "Odo"}))
	assert.Equal(t, "odo", duplicate([]string{"Wren", "Odo", "odo"}))
}
