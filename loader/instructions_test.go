package loader

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

// A missing instance_max is a mistake, not "unlimited": for an object it
// meant another every reset, and for a mob never.
func TestLoadZoneInstructions_needsInstanceMax(t *testing.T) {
	c := lootContent(t)
	fsys := fstest.MapFS{
		"caves/instructions.json": {Data: []byte(`[{"type": "CreateObject", "object_id": "bone", "room_id": "pit"}]`)},
	}
	assert.ErrorContains(t, c.loadZoneInstructions(fsys), "instance_max")
}
