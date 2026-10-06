package spaces

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/lock"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

// a zone with a loft, a grain bin that shuts, and a key to go in it
func lootedZone() (*Zone, *Room) {
	z := NewZone("mill", "The Drowned Mill", rules.ZoneResetAlways, 0)
	z.Power = rules.PowerBand{Min: 4, Max: 7}
	loft := NewRoom(z, "loft", "The Grain Loft", "")
	z.AddRoom(loft)
	bin := object.NewDefinition("grain_bin", "grain bin", "mill", rules.ObjectCategoryOther, nil,
		"a grain bin", "A grain bin stands here.", rules.SlotNone, rules.ArmorTypeNone, nil)
	bin.Container = &object.ContainerSpec{Initial: lock.State{Closed: true}}
	z.AddObjectDefinition(bin)
	z.AddObjectDefinition(object.NewDefinition("key", "small iron key", "mill", rules.ObjectCategoryOther, nil,
		"a small iron key", "A small iron key is here.", rules.SlotNone, rules.ArmorTypeNone, nil))
	seven := 7
	z.AddCommand(CreateObject{ObjectDefinitionId: "grain_bin", RoomId: "loft", InstanceMax: 1})
	z.AddCommand(CreateObject{ObjectDefinitionId: "key", RoomId: "loft", ContainerId: "grain_bin", InstanceMax: 1, Power: &seven})
	z.AddCommand(CreateObject{ObjectDefinitionId: "key", RoomId: "loft", InstanceMax: 2})
	return z, loft
}

func count(l *object.List) int {
	n := 0
	for range l.All() {
		n++
	}
	return n
}

// Resets top up to instance_max, on the floor and in a container, rather
// than adding one more each time.
func TestReset_topsUpRatherThanPilingUp(t *testing.T) {
	z, loft := lootedZone()
	for range 3 {
		require.Empty(t, z.Reset(NewOccupancy()))
	}

	assert.Equal(t, 1, countOf(loft.Inventory, "grain_bin"))
	assert.Equal(t, 2, countOf(loft.Inventory, "key"), "the floor's two")
	bin := firstOf(loft.Inventory, "grain_bin")
	assert.Equal(t, 1, count(bin.Contents), "and the one in the bin")
}

func TestReset_powerAndContainer(t *testing.T) {
	z, loft := lootedZone()
	require.Empty(t, z.Reset(NewOccupancy()))

	bin := firstOf(loft.Inventory, "grain_bin")
	inBin := firstOf(bin.Contents, "key")
	assert.Equal(t, 7, inBin.Power, "what the instruction said")
	assert.Equal(t, 4, firstOf(loft.Inventory, "key").Power, "otherwise the bottom of the band")
}

// a reset shuts a chest someone left open, and puts back what was taken
func TestReset_closesTheChestAgain(t *testing.T) {
	z, loft := lootedZone()
	require.Empty(t, z.Reset(NewOccupancy()))
	bin := firstOf(loft.Inventory, "grain_bin")
	require.NoError(t, bin.Lock.Open())
	require.NoError(t, bin.Contents.Remove(firstOf(bin.Contents, "key")))

	require.Empty(t, z.Reset(NewOccupancy()))

	assert.True(t, bin.Closed())
	assert.Equal(t, 1, count(bin.Contents))
}

func TestReset_noSuchContainer(t *testing.T) {
	z, _ := lootedZone()
	z.Commands = []ZoneCommand{CreateObject{ObjectDefinitionId: "key", RoomId: "loft", ContainerId: "strongbox", InstanceMax: 1}}
	assert.NotEmpty(t, z.Reset(NewOccupancy()))
}
