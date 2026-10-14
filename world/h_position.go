package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/player"
)

var positionByVerb = map[string]player.Position{
	"stand": player.Standing,
	"sit":   player.Sitting,
	"rest":  player.Resting,
	"sleep": player.Sleeping,
}

// handlePosition is sit, rest, sleep, stand and wake. Off your feet you heal
// faster and can't walk; asleep you can do little but wake. Nobody sits down
// in the middle of a fight -- and a fight brings anyone off their feet back
// onto them (standUp, from joinFight).
func (w *World) handlePosition(msg *gameserver.HandlerParameter, cmd command.Position) {
	p := msg.Player
	to := positionByVerb[cmd.To]
	if cmd.Wake {
		if p.Position() != player.Sleeping {
			msg.Fail(event.AlreadyPosition)
			return
		}
		to = player.Standing
	}
	if p.Position() == to {
		msg.Fail(event.AlreadyPosition)
		return
	}
	if to != player.Standing && w.fightLedger.InFight(p) {
		msg.Fail(event.InAFight)
		return
	}
	woke := p.Position() == player.Sleeping
	p.SetPosition(to)
	w.playerRoom(p).Send(event.PositionChanged{Actor: p.Name(), To: to.String(), Woke: woke})
}

// standUp puts a player drawn into a fight back on their feet, and tells the
// room.
func (w *World) standUp(p *player.Player) {
	if p.Position() == player.Standing {
		return
	}
	woke := p.Position() == player.Sleeping
	p.SetPosition(player.Standing)
	if room := w.playerRoom(p); room != nil {
		room.Send(event.PositionChanged{Actor: p.Name(), Woke: woke})
	}
}

// awake is what a sleeping player may still do: wake or stand, look over
// themselves and their group, and leave.
func awake(cmd command.Command) bool {
	switch c := cmd.(type) {
	case command.Position:
		return c.Wake || c.To == "stand"
	case command.Logout, command.Stat, command.Inventory, command.ShowEquipment,
		command.Abilities, command.Group, command.Toggle, command.Who,
		command.Color, command.ScreenReader:
		return true
	}
	return false
}
