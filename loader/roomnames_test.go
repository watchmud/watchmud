package loader

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The bots find their way by the room name a player reads -- the smoke walk,
// the hunting grounds, a wanderer's sense of where it came from -- so two
// rooms with one name would send them the wrong way. Not a rule of the game,
// a rule of the content while bots navigate like that.
func TestRoomNames_uniqueInTheWorld(t *testing.T) {
	c, err := LoadContent(os.DirFS("../content"))
	require.NoError(t, err)

	seen := map[string]string{}
	for _, zoneId := range c.zoneNames() {
		for _, r := range c.Zones[zoneId].Rooms {
			where := zoneId + "/" + r.Id
			if other, dup := seen[r.Name]; dup {
				assert.Failf(t, "room name used twice", "%q is %s and %s", r.Name, other, where)
			}
			seen[r.Name] = where
		}
	}
}
