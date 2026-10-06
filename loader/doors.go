package loader

import (
	"fmt"

	"github.com/watchmud/watchmud/lock"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
)

// pendingDoor is a door as rooms.json declared it, on the exit from room going
// dir, waiting for its key to be resolvable.
type pendingDoor struct {
	zone, room string
	dir        rules.Direction
	entry      doorEntry
}

// hangDoors puts every declared door in its exit and in the exit back, so both
// sides share it, after checking it makes sense: a locked door is closed and
// has a key, the key is a real object, and nothing declares a door twice.
func (c *Content) hangDoors() error {
	for _, p := range c.pendingDoors {
		where := fmt.Sprintf("%s/%s going %s", p.zone, p.room, p.dir)
		e := p.entry
		key, err := c.lockKey(p.zone, e.Closed, e.Locked, e.Key)
		if err != nil {
			return fmt.Errorf("door %s: %w", where, err)
		}
		name := e.Name
		if name == "" {
			name = "door"
		}

		zone := c.Zones[p.zone]
		room := zone.Rooms[p.room]
		if room.DoorTo(p.dir) != nil {
			return fmt.Errorf("door %s: there's a door there already -- declare a door on one side only, and both sides share it", where)
		}
		door := spaces.NewDoor(name, e.Aliases, lock.New(lock.State{Closed: e.Closed, Locked: e.Locked}, key))
		room.SetDoor(p.dir, door)
		door.Sides = append(door.Sides, room)
		if far := room.DestinationRoom(p.dir); far != nil {
			door.Sides = append(door.Sides, far)
		}
		if far := room.DestinationRoom(p.dir); far.DestinationRoom(p.dir.Opposite()) == room {
			if far.DoorTo(p.dir.Opposite()) != nil {
				return fmt.Errorf("door %s: there's a door there already -- declare a door on one side only, and both sides share it", where)
			}
			far.SetDoor(p.dir.Opposite(), door)
		}
		zone.Doors = append(zone.Doors, door)
	}
	c.pendingDoors = nil
	return nil
}
