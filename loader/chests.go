package loader

import (
	"fmt"

	"github.com/watchmud/watchmud/lock"
	"github.com/watchmud/watchmud/object"
)

// pendingChest is a container definition waiting, like a door, for every
// zone's objects so its key can be one of them.
type pendingChest struct {
	zone  string
	defn  *object.Definition
	entry containerEntry
}

// fitChests gives each container definition its lid, with the checks a door's
// lock gets: a locked one is closed and has a key, and the key is an object.
func (c *Content) fitChests() error {
	for _, p := range c.pendingChests {
		where := fmt.Sprintf("%s/%s", p.zone, p.defn.ObjectId.DefinitionId)
		if p.entry.Capacity < 0 {
			return fmt.Errorf("container %s: capacity %d", where, p.entry.Capacity)
		}
		if p.entry.Portable && (p.entry.Closed || p.entry.Locked || p.entry.Key != "") {
			// a lid's state would have to be saved with the carrier; a bag is
			// always open
			return fmt.Errorf("container %s: a portable container has no lid or lock", where)
		}
		if p.entry.Portable && p.defn.NoTake() {
			return fmt.Errorf("container %s: portable, and noTake", where)
		}
		key, err := c.lockKey(p.zone, p.entry.Closed, p.entry.Locked, p.entry.Key)
		if err != nil {
			return fmt.Errorf("container %s: %w", where, err)
		}
		p.defn.Container = &object.ContainerSpec{
			Initial:  lock.State{Closed: p.entry.Closed, Locked: p.entry.Locked},
			Key:      key,
			Portable: p.entry.Portable,
			Capacity: p.entry.Capacity,
		}
	}
	c.pendingChests = nil
	return nil
}

// lockKey checks a lock's starting state and resolves its key to "zone/id":
// what a door and a chest both need.
func (c *Content) lockKey(zone string, closed, locked bool, ref string) (string, error) {
	if locked && !closed {
		return "", fmt.Errorf("locked but not closed")
	}
	if locked && ref == "" {
		return "", fmt.Errorf("locked, and no key to open it")
	}
	if ref == "" {
		return "", nil
	}
	defn, err := c.objectRef(zone, ref)
	if err != nil {
		return "", fmt.Errorf("key %w", err)
	}
	return defn.ObjectId.Ref(), nil
}
