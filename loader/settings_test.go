package loader

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSettings(t *testing.T) {
	fsys := fstest.MapFS{"settings.json": {Data: []byte(`{
		"void": { "zone_id": "void", "room_id": "void" },
		"start": { "zone_id": "wrathrock", "room_id": "temple_square" },
		"donation": { "zone_id": "wrathrock", "room_id": "donation_room" },
		"player-death": { "zone_id": "wrathrock", "room_id": "chapel" },
		"welcome": "Head south."
	}`)}}

	s, err := LoadSettings(fsys)
	require.NoError(t, err)

	assert.Equal(t, RoomRef{ZoneId: "void", RoomId: "void"}, s.Void)
	assert.Equal(t, RoomRef{ZoneId: "wrathrock", RoomId: "temple_square"}, s.Start)
	assert.Equal(t, RoomRef{ZoneId: "wrathrock", RoomId: "donation_room"}, s.Donation)
	assert.Equal(t, RoomRef{ZoneId: "wrathrock", RoomId: "chapel"}, s.PlayerDeath)
	assert.Equal(t, "Head south.", s.Welcome)
}
