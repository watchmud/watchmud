package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringToDirection_Successful(t *testing.T) {
	assertDirection := func(dir Direction, s string) {
		parsed, err := ParseDirection(s)
		if err != nil {
			t.Error(err)
		} else {
			assert.Equal(t, dir, parsed)
		}
	}
	assertDirection(DirectionNorth, "n")
	assertDirection(DirectionNorth, "north")
	assertDirection(DirectionNorth, "NoRtH")
	assertDirection(DirectionEast, "e")
	assertDirection(DirectionWest, "w")
	assertDirection(DirectionSouth, "s")
	assertDirection(DirectionUp, "U")
	assertDirection(DirectionUp, "up")
	assertDirection(DirectionDown, "D")
}

func TestAbbreviationToDirection_EmptyString(t *testing.T) {
	_, err := ParseDirection("")
	if err == nil {
		t.Error("Expected error")
	}
}

func TestStringToDirection_Unknown(t *testing.T) {
	_, err := ParseDirection("asdf")
	if err == nil {
		t.Error("expected error")
	}
}

func TestAbbreviationToString(t *testing.T) {
	doit := func(dir string, expected Direction) {
		d, err := ParseDirectionPrefix(dir)
		assert.Equal(t, expected, d)
		assert.Nil(t, err)
	}

	doit("n", DirectionNorth)
	doit("s", DirectionSouth)
	doit("e", DirectionEast)
	doit("w", DirectionWest)
	doit("u", DirectionUp)
	doit("d", DirectionDown)

	doit("N", DirectionNorth)
	doit("S", DirectionSouth)
	doit("E", DirectionEast)
	doit("W", DirectionWest)
	doit("U", DirectionUp)
	doit("D", DirectionDown)
}

func TestAbbreviationToString_IsUnknown(t *testing.T) {
	_, err := ParseDirectionPrefix("x")
	assert.NotNil(t, err)
}

func TestAbbreviationToString_IsTooBig(t *testing.T) {
	_, err := ParseDirectionPrefix("asdlfkjasdlf")
	assert.NotNil(t, err)
}

func TestDirectionToAbbreviation(t *testing.T) {
	doit := func(dir Direction, expected string) {
		str := dir.Abbrev()
		assert.Equal(t, expected, str)
	}

	doit(DirectionNorth, "n")
	doit(DirectionSouth, "s")
	doit(DirectionEast, "e")
	doit(DirectionWest, "w")
	doit(DirectionUp, "u")
	doit(DirectionDown, "d")
}

func TestDirectionToString(t *testing.T) {
	doit := func(dir Direction, expected string) {
		str := dir.String()
		assert.Equal(t, expected, str)
	}

	doit(DirectionNorth, "North")
	doit(DirectionSouth, "South")
	doit(DirectionEast, "East")
	doit(DirectionWest, "West")
	doit(DirectionUp, "Up")
	doit(DirectionDown, "Down")
}

func TestDirection_opposite(t *testing.T) {
	for _, d := range AllUsableDirections {
		assert.Equal(t, d, d.Opposite().Opposite(), "%s and back", d)
		assert.NotEqual(t, d, d.Opposite())
	}
	assert.Equal(t, DirectionSouth, DirectionNorth.Opposite())
	assert.Equal(t, DirectionNone, DirectionNone.Opposite())
}
