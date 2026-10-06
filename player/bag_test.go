package player

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

// bagDefs knows a satchel and a knife, and the satchel may stop being a bag.
type bagDefs struct {
	satchel, knife *object.Definition
}

func (d bagDefs) ObjectDefinition(zoneId, definitionId string) (*object.Definition, bool) {
	switch {
	case zoneId == "wrathrock" && definitionId == "satchel":
		return d.satchel, true
	case zoneId == "wrathrock" && definitionId == "knife":
		return d.knife, true
	}
	return nil, false
}

func newBagDefs(t *testing.T) bagDefs {
	t.Helper()
	satchel := object.NewDefinition("satchel", "satchel", "wrathrock", rules.ObjectCategoryOther, nil,
		"a satchel", "A satchel is here.", rules.SlotNone, rules.ArmorTypeNone, nil)
	satchel.Container = &object.ContainerSpec{Portable: true}
	return bagDefs{satchel: satchel, knife: object.MakeTestKnife(t).Definition}
}

func packed(t *testing.T, defs bagDefs) (*Record, uuid.UUID) {
	t.Helper()
	p := NewTestPlayer(uuid.New(), "packed", nil)
	bag := object.NewInstance(uuid.New(), defs.satchel)
	knife := object.NewInstance(uuid.New(), defs.knife)
	require.NoError(t, bag.Contents.Add(knife))
	require.NoError(t, p.Inventory().Add(bag))
	return p.Record(), knife.Id
}

// What's in a bag logs out and back in inside it.
func TestRecord_bagContentsRoundTrip(t *testing.T) {
	defs := newBagDefs(t)
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)

	rec, knifeId := packed(t, defs)
	require.Len(t, rec.Inventory, 1)
	require.Len(t, rec.Inventory[0].Contents, 1)

	back, err := FromRecord(rec, &Recorder{}, cat, defs)
	require.NoError(t, err)
	var bag *object.Instance
	for i := range back.Inventory().All() {
		bag = i
	}
	require.NotNil(t, bag)
	_, inBag := bag.Contents.Get(knifeId)
	assert.True(t, inBag)
	assert.Equal(t, 1, back.Inventory().Len(), "the knife isn't loose as well")
}

// A satchel content no longer calls a bag doesn't take the knife with it.
func TestRecord_bagThatIsNoLongerABag(t *testing.T) {
	defs := newBagDefs(t)
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)
	rec, knifeId := packed(t, defs)

	defs.satchel.Container = nil
	back, err := FromRecord(rec, &Recorder{}, cat, defs)
	require.NoError(t, err)
	_, loose := back.Inventory().Get(knifeId)
	assert.True(t, loose)
}
