package object

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/watchmud/watchmud/lock"
	"github.com/watchmud/watchmud/rules"
)

// Every chest made from one definition has its own contents and its own lid.
func TestNewInstance_container(t *testing.T) {
	d := NewDefinition("strongbox", "strongbox", "mill", rules.ObjectCategoryOther, nil,
		"a strongbox", "A strongbox is here.", rules.SlotNone, rules.ArmorTypeNone, nil)
	d.Container = &ContainerSpec{Initial: lock.State{Closed: true, Locked: true}, Key: "mill/strongbox_key"}

	a, b := NewInstance(uuid.New(), d), NewInstance(uuid.New(), d)

	assert.NotNil(t, a.Contents)
	assert.True(t, a.Closed())
	assert.Equal(t, "mill/strongbox_key", a.Lock.Key)
	_ = a.Lock.Unlock(true)
	_ = a.Lock.Open()
	assert.False(t, a.Closed())
	assert.True(t, b.Closed(), "its own lock")
	assert.NotSame(t, a.Contents, b.Contents)
}

func TestNewInstance_notAContainer(t *testing.T) {
	d := NewDefinition("knife", "knife", "mill", rules.ObjectCategoryWeapon, nil,
		"a knife", "A knife is here.", rules.SlotWield, rules.ArmorTypeNone, nil)
	k := NewInstance(uuid.New(), d)
	assert.Nil(t, k.Contents)
	assert.Nil(t, k.Lock)
	assert.False(t, k.Closed())
}
