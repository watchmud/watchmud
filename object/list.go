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
	items *ordered.List[uuid.UUID, *Instance]
}

func NewList() *List {
	return &List{items: ordered.NewList(func(i *Instance) uuid.UUID { return i.Id })}
}

// All the instances, in the order they went in.
func (l *List) All() iter.Seq[*Instance] { return l.items.All() }

func (l *List) Len() int { return l.items.Len() }

// Get the instance with this id.
func (l *List) Get(id uuid.UUID) (*Instance, bool) { return l.items.Get(id) }

func (l *List) FindAll(target string) []*Instance { return ordered.FindAll(l.items, target) }

func (l *List) Add(inst *Instance) error {
	if err := l.items.Add(inst); err != nil {
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
// end of from instead of where it was.
func Move(inst *Instance, from, to *List) error {
	if err := from.Remove(inst); err != nil {
		return err
	}
	if err := to.Add(inst); err != nil {
		return errors.Join(err, from.Add(inst))
	}
	return nil
}
