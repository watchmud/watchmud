package object

import (
	"time"
	"uuid"

	"github.com/watchmud/watchmud/lock"
)

// Instance of the Definitions in the world around you.
// That ShinySword in your hand has certain properties, some of
// which were inherited by what it means to be a ShinySword (Definition)
// and others which have happened to that particular instance
// (soul-bound to you, made invisible, with some damage to the hilt).
type Instance struct {
	Id         uuid.UUID
	Definition *Definition

	// Durability is what this particular one has left, counting down from
	// Definition.MaxDurability. See durability.go.
	Durability int

	// Power is how strong this particular one is, and it belongs to the
	// instance rather than the definition so one rusty sword can serve every
	// tier: power 5 off a goblin, power 15 off an ogre. Zero is the bottom.
	// Whatever makes the instance sets it; see LEVELS.md.
	Power int

	// Contents is what's inside, for a container; nil for anything that
	// isn't one. Corpses and chests are.
	Contents *List

	// Lock is a chest's lid: nil for a container that's always open (a
	// corpse) and for anything that isn't a container.
	Lock *lock.Lock

	// Coins is what a container holds besides its Contents: coins are a
	// count, not things, so they don't go in the list. Only a corpse has
	// any, and they go with it when it crumbles.
	Coins int

	// DecaysAt is when this crumbles away, whatever it's holding; the zero
	// time means never. Only corpses decay so far.
	DecaysAt time.Time
}

// IdStr from the Thing interface
// TODO figure out if this can be removed
func (i *Instance) IdStr() string { return i.Id.String() }

func (i *Instance) Matches(target string) bool {
	return i.Definition.Matches(target)
}

// NewInstance of a definition, brand new: full durability. Anything restoring
// one that has already been used -- a save file -- sets Durability afterwards.
func NewInstance(id uuid.UUID, d *Definition) *Instance {
	inst := &Instance{
		Id:         id,
		Definition: d,
		Durability: d.MaxDurability,
	}
	if d.Container != nil {
		inst.Contents = NewList()
		inst.Lock = lock.New(d.Container.Initial, d.Container.Key)
	}
	return inst
}

// Closed is whether this is a container whose lid is shut: nothing in it can
// be seen or taken.
func (i *Instance) Closed() bool {
	return i.Lock != nil && i.Lock.Closed
}
