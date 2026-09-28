package object

import (
	"errors"
	"fmt"
	"iter"
	"uuid"

	"github.com/watchmud/watchmud/ordered"
)

// List is a container of object instances - a room's floor,  what a player
// is carrying, what's inside a corpse -- in the order things went in. One type
// for all of them, so moving a thing from one to another is the same code
// wherever it happens; see Move.
//
// Add and Remove return errors, wrapping ordered.ErrDuplicate/ErrNotFound. With
// one list per place and Move as the way between them, a duplicate or a miss is
// a bug worth hearing about, not noise.
type List struct {
	items       *ordered.List[uuid.UUID, *Instance]
	newestFirst bool
}

func NewList() *List {
	return &List{items: ordered.NewList(func(i *Instance) uuid.UUID { return i.Id })}
}

// NewFloor is a list that holds its newest arrival first: a room's floor. Bare
// "corpse" is then the one that just fell, not the one about to decay, and
// "2.corpse" the one before it -- the same order "look" shows, so the numbers
// a player reads are the numbers they type. The order belongs to the place
// rather than to whoever adds to it, so Move, drop, corpses, zone resets and
// wizard loads all get it without asking.
func NewFloor() *List {
	l := NewList()
	l.newestFirst = true
	return l
}

// All the instances, in the order they went in (newest first, for a floor).
func (l *List) All() iter.Seq[*Instance] { return l.items.All() }

func (l *List) Len() int { return l.items.Len() }

// Get the instance with this id.
func (l *List) Get(id uuid.UUID) (*Instance, bool) { return l.items.Get(id) }

func (l *List) FindAll(target string) []*Instance { return ordered.FindAll(l.items, target) }

func (l *List) Add(inst *Instance) error {
	add := l.items.Add
	if l.newestFirst {
		add = l.items.Push
	}
	if err := add(inst); err != nil {
		return fmt.Errorf("add %s: %w", inst.Definition.Name, err)
	}
	return nil
}

func (l *List) Remove(inst *Instance) error {
	if err := l.items.Remove(inst); err != nil {
		return fmt.Errorf("remove %s: %w", inst.Definition.Name, err)
	}
	return nil
}

// Move inst out of from and into to. If it can't go in, it goes back
// where it came from, so a failed move loses nothing - at worst it's at the
// end (or, for a floor, the front) of from instead of where it was.
func Move(inst *Instance, from, to *List) error {
	if err := from.Remove(inst); err != nil {
		return err
	}
	if err := to.Add(inst); err != nil {
		return errors.Join(err, from.Add(inst))
	}
	return nil
}
