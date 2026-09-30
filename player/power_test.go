package player

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

func powered(t *testing.T, defs durabilityDefs, power int) *Record {
	t.Helper()
	p := NewTestPlayer(uuid.New(), "geared", nil)
	knife := object.NewInstance(uuid.New(), defs.knife)
	knife.Power = power
	require.NoError(t, p.Inventory().Add(knife))
	p.Equipment().Equip(rules.SlotWield, knife)
	return p.Record()
}

// Power belongs to the instance, so the knife you logged out with is the knife
// you log back in with -- not a fresh one off the definition.
func TestRecord_powerRoundTrips(t *testing.T) {
	defs := newDurabilityDefs(t, rules.Indestructible)
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)

	rec := powered(t, defs, 7)
	require.NotNil(t, rec.Inventory[0].Power)
	assert.Equal(t, 7, *rec.Inventory[0].Power)

	back, err := FromRecord(rec, &Recorder{}, cat, defs)
	require.NoError(t, err)
	assert.Equal(t, 7, back.Equipment().At(rules.SlotWield).Power)
}

// A record written before power existed says nothing about it, and that is
// power 0 -- the bottom, which is what everything was before power existed.
func TestRecord_missingPowerIsZero(t *testing.T) {
	defs := newDurabilityDefs(t, rules.Indestructible)
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)

	rec := powered(t, defs, 7)
	rec.Inventory[0].Power = nil // an older save

	back, err := FromRecord(rec, &Recorder{}, cat, defs)
	require.NoError(t, err)
	assert.Equal(t, 0, back.Equipment().At(rules.SlotWield).Power)
}

// Power 0 writes no number, the same as gear that doesn't wear out writes no
// durability: absent and zero already mean the same thing on the way back.
func TestRecord_zeroPowerRecordsNothing(t *testing.T) {
	rec := powered(t, newDurabilityDefs(t, rules.Indestructible), 0)
	assert.Nil(t, rec.Inventory[0].Power)
}

// Nothing makes negative power, so a record claiming it is damage, not content;
// it comes back as the floor rather than dragging a player's average below it.
func TestRecord_negativePowerIsClampedToZero(t *testing.T) {
	defs := newDurabilityDefs(t, rules.Indestructible)
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)

	rec := powered(t, defs, 7)
	*rec.Inventory[0].Power = -3

	back, err := FromRecord(rec, &Recorder{}, cat, defs)
	require.NoError(t, err)
	assert.Equal(t, 0, back.Equipment().At(rules.SlotWield).Power)
}

// A player's power is what they're wearing, read fresh every time: put on
// something better and the next swing knows.
func TestPlayer_powerIsTheirGear(t *testing.T) {
	defs := newDurabilityDefs(t, rules.Indestructible)
	p := NewTestPlayer(uuid.New(), "geared", nil)
	assert.Equal(t, 0, p.Power(), "nothing on")

	knife := object.NewInstance(uuid.New(), defs.knife)
	knife.Power = 9
	require.NoError(t, p.Inventory().Add(knife))
	p.Equipment().Equip(rules.SlotWield, knife)
	assert.Equal(t, 9, p.Power())

	p.Equipment().Unequip(rules.SlotWield)
	assert.Equal(t, 0, p.Power())
}
