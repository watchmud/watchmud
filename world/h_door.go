package world

import (
	"errors"

	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/lock"
	"github.com/watchmud/watchmud/player"
)

// handleDoor is open, close, lock and unlock: find the door the player means,
// try the change on its lock, and tell both rooms it joins. The far side hears
// the door, not who did it.
func (w *World) handleDoor(msg *gameserver.HandlerParameter, target string, change event.DoorChange) {
	if target == "" {
		msg.Fail(event.NoTarget)
		return
	}
	room := w.playerRoom(msg.Player)
	dir, door, found := room.FindDoor(target)
	if !found {
		msg.Fail(event.NoDoor)
		return
	}

	var err error
	switch change {
	case event.DoorOpened:
		err = door.Open()
	case event.DoorClosed:
		err = door.Close()
	case event.DoorLocked:
		err = door.Lock.Lock(carriesKey(msg.Player, door.Key))
	case event.DoorUnlocked:
		err = door.Unlock(carriesKey(msg.Player, door.Key))
	}
	if err != nil {
		msg.Fail(lockFailure(err))
		return
	}

	room.Send(event.DoorChanged{Actor: msg.Player.Name(), Door: door.Name, Direction: dir, Change: change})
	if far := room.DestinationRoom(dir); far != nil && far != room {
		far.Send(event.DoorChanged{Door: door.Name, Direction: dir.Opposite(), Change: change})
	}
}

// carriesKey is whether anything the player has -- carried or worn, since
// what's worn is in the inventory too -- is the object that key names.
func carriesKey(p *player.Player, key string) bool {
	if key == "" {
		return false
	}
	for inst := range p.Inventory().All() {
		if inst.Definition.ObjectId.Ref() == key {
			return true
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
