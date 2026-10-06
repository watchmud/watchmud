package world

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/object"
)

// findContainer is the container a player named: one they carry -- a bag --
// first, then one on the floor of the room they're in.
func (w *World) findContainer(msg *gameserver.HandlerParameter, name string) (*object.Instance, bool) {
	target, err := parseTarget(name)
	if err != nil {
		log.Debug().Err(err).Str("player", msg.Player.Name()).Str("container", name).Msg("can't parse container")
		msg.Fail(event.ParseError)
		return nil, false
	}
	found := targetsIn(target, msg.Player.Inventory().All())
	if len(found) == 0 || found[0].Contents == nil {
		found = targetsIn(target, w.playerRoom(msg.Player).Inventory.All())
	}
	if len(found) == 0 {
		msg.Fail(event.TargetNotFound)
		return nil, false
	}
	if found[0].Contents == nil {
		msg.Fail(event.NotAContainer)
		return nil, false
	}
	if found[0].Closed() {
		msg.Fail(event.ContainerClosed)
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
	// coins aren't things: "get coins from corpse", "get 5 coins from
	// corpse", or they come along with "get all"
	everything := target.All && target.Name == ""
	if isCoins(target.Name) {
		if container.Coins == 0 {
			msg.Fail(event.NotInContainer)
			return
		}
		w.takeCoins(msg, container, target.Quantity)
		return
	}
	items := targetsIn(target, container.Contents.All())
	if len(items) == 0 && !(everything && container.Coins > 0) {
		if everything {
			msg.Fail(event.ContainerEmpty)
		} else {
			msg.Fail(event.NotInContainer)
		}
		return
	}

	room := w.playerRoom(msg.Player)
	if everything && container.Coins > 0 {
		w.takeCoins(msg, container, 0)
	}
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

// isCoins is whether a target names the coins rather than a thing.
func isCoins(name string) bool {
	return name == "coins" || name == "coin"
}

// takeCoins moves coins from a container to the player's purse: up to want
// of them, or all when want is zero. The room sees it the way it sees any
// other get.
func (w *World) takeCoins(msg *gameserver.HandlerParameter, container *object.Instance, want int) {
	n := container.Coins
	if want > 0 {
		n = min(n, want)
	}
	container.Coins -= n
	msg.Player.AddCoins(n)
	w.playerRoom(msg.Player).Send(event.Got{
		Actor: msg.Player.Name(),
		Item:  coinsText(n),
		From:  container.Definition.ShortDescription,
	})
}

// coinsText is a number of coins as a player reads it.
func coinsText(n int) string {
	if n == 1 {
		return "1 coin"
	}
	return fmt.Sprintf("%d coins", n)
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
		Coins:     container.Coins,
	})
}
