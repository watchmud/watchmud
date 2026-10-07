package world

import (
	"errors"

	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/lock"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/spaces"
)

// handleDoor is open, close, lock and unlock, for a door or a chest: a door
// first (by name or direction), then a container with a lid on the floor. The
// change is tried on its lock.Lock, and whoever can see it is told.
func (w *World) handleDoor(msg *gameserver.HandlerParameter, target string, change event.DoorChange) {
	if target == "" {
		msg.Fail(event.NoTarget)
		return
	}
	room := w.playerRoom(msg.Player)
	if dir, door, found := room.FindDoor(target); found {
		if err := tryLock(door.Lock, msg.Player, change); err != nil {
			msg.Fail(lockFailure(err))
			return
		}
		room.Send(event.DoorChanged{Actor: msg.Player.Name(), Door: door.Name, Direction: dir, Change: change})
		if far := room.DestinationRoom(dir); far != nil && far != room {
			far.Send(event.DoorChanged{Door: door.Name, Direction: dir.Opposite(), Change: change})
		}
		return
	}
	chest := w.findLidded(room, target)
	if chest == nil {
		msg.Fail(event.NoDoor)
		return
	}
	if err := tryLock(chest.Lock, msg.Player, change); err != nil {
		msg.Fail(lockFailure(err))
		return
	}
	room.Send(event.ContainerChanged{Actor: msg.Player.Name(), Container: chest.Definition.Name, Change: change})
}

// findLidded is a container with a lid on the room's floor that target names,
// or nil. The target grammar is get's -- "chest", "2.chest".
func (w *World) findLidded(room *spaces.Room, target string) *object.Instance {
	t, err := parseTarget(target)
	if err != nil {
		return nil
	}
	// only lidded things are counted, before "2." picks among them: a
	// tinderbox dropped beside the strongbox mustn't answer for "box"
	lidded := func(yield func(*object.Instance) bool) {
		for inst := range room.Inventory.All() {
			if inst.Lock != nil && !yield(inst) {
				return
			}
		}
	}
	found := targetsIn(t, lidded)
	if len(found) == 0 {
		return nil
	}
	return found[0]
}

// tryLock is one change to a lock, with the player's key if they carry it.
func tryLock(l *lock.Lock, p *player.Player, change event.DoorChange) error {
	switch change {
	case event.DoorOpened:
		return l.Open()
	case event.DoorClosed:
		return l.Close()
	case event.DoorLocked:
		return l.Lock(carriesKey(p, l.Key))
	case event.DoorUnlocked:
		return l.Unlock(carriesKey(p, l.Key))
	}
	return nil
}

// carriesKey is whether anything the player has -- carried, worn (what's
// worn is in the inventory too), or in a bag -- is the object that key names.
func carriesKey(p *player.Player, key string) bool {
	if key == "" {
		return false
	}
	for inst := range p.Inventory().All() {
		if inst.Definition.ObjectId.Ref() == key {
			return true
		}
		// a key in a bag is still carried
		if inst.Contents != nil {
			for in := range inst.Contents.All() {
				if in.Definition.ObjectId.Ref() == key {
					return true
				}
			}
		}
	}
	return false
}

// lockFailure is why a lock said no, as the player is told it.
func lockFailure(err error) event.ResultCode {
	switch {
	case errors.Is(err, lock.ErrAlreadyOpen):
		return event.AlreadyOpen
	case errors.Is(err, lock.ErrAlreadyClosed):
		return event.AlreadyClosed
	case errors.Is(err, lock.ErrAlreadyLocked):
		return event.AlreadyLocked
	case errors.Is(err, lock.ErrNotLocked):
		return event.NotLocked
	case errors.Is(err, lock.ErrLocked):
		return event.Locked
	case errors.Is(err, lock.ErrNotClosed):
		return event.NotClosed
	case errors.Is(err, lock.ErrNoKey):
		return event.NoKey
	case errors.Is(err, lock.ErrHasNoKeyhole):
		return event.NoKeyhole
	}
	return event.InternalError
}
