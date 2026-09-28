package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/object"
)

// findContainer is the container a player named, on the floor of the room
// they're in -- the only place a corpse can be. A bag in your inventory will
// want the inventory searched too.
func (w *World) findContainer(msg *gameserver.HandlerParameter, name string) (*object.Instance, bool) {
	target, err := parseTarget(name)
	if err != nil {
		log.Debug().Err(err).Str("player", msg.Player.Name()).Str("container", name).Msg("can't parse container")
		msg.Fail(event.ParseError)
		return nil, false
	}
	found := targetsIn(target, w.playerRoom(msg.Player).Inventory.All())
	if len(found) == 0 {
		msg.Fail(event.TargetNotFound)
		return nil, false
	}
	if found[0].Contents == nil {
		msg.Fail(event.NotAContainer)
		return nil, false
	}
	return found[0], true
}

// getFrom is "get <item> from <container>", and "get all from corpse".
func (w *World) getFrom(msg *gameserver.HandlerParameter, cmd command.Get) {
	container, ok := w.findContainer(msg, cmd.From)
	if !ok {
		return
	}
	target, err := parseTarget(cmd.Target)
	if err != nil {
		log.Debug().Err(err).Str("player", msg.Player.Name()).Str("target", cmd.Target).Msg("get from: can't parse target")
		msg.Fail(event.ParseError)
		return
	}
	items := targetsIn(target, container.Contents.All())
	if len(items) == 0 {
		if target.All && target.Name == "" {
			msg.Fail(event.ContainerEmpty)
		} else {
			msg.Fail(event.NotInContainer)
		}
		return
	}

	room := w.playerRoom(msg.Player)
	for _, item := range items {
		if err := object.Move(item, container.Contents, msg.Player.Inventory()); err != nil {
			log.Error().Err(err).Str("player", msg.Player.Name()).Str("room", room.Location().String()).Msg("get")
			msg.Fail(event.RemoveFromRoomError)
			return
		}
		room.Send(event.Got{
			Actor: msg.Player.Name(),
			Item:  item.Definition.ShortDescription,
			From:  container.Definition.ShortDescription,
		})
	}
}

// lookIn is "look in <container>".
func (w *World) lookIn(msg *gameserver.HandlerParameter, name string) {
	if name == "" {
		msg.Fail(event.NoTarget)
		return
	}
	container, ok := w.findContainer(msg, name)
	if !ok {
		return
	}
	var items []event.ContainedItem
	for item := range container.Contents.All() {
		items = append(items, event.ContainedItem{
			ShortDescription: item.Definition.ShortDescription,
			Power:            item.Power,
		})
	}
	msg.Player.Send(event.ContainerContents{
		Container: container.Definition.ShortDescription,
		Items:     items,
	})
}
