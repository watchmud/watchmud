package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/script"
)

// handleWhisper is whisper and ask: words for one in the room, the rest of
// the room seeing only that something was said. To a scripted mob it's heard
// as a say would be, by that mob alone -- "ask shopkeeper about prices".
func (w *World) handleWhisper(msg *gameserver.HandlerParameter, cmd command.Whisper) {
	if cmd.To == "" {
		msg.Fail(event.NoTarget)
		return
	}
	if cmd.Value == "" {
		msg.Fail(event.NoValue)
		return
	}
	room := w.playerRoom(msg.Player)
	if p, found := room.FindPlayer(cmd.To); found && p != msg.Player {
		room.Send(event.Whispered{From: msg.Player.Name(), To: p.Name(), Value: cmd.Value, Ask: cmd.Ask})
		return
	}
	if mob, found := room.FindMobile(cmd.To); found {
		room.Send(event.Whispered{From: msg.Player.Name(), To: "the " + mob.Name(), Value: cmd.Value, Ask: cmd.Ask})
		if mob.Definition.Script != "" {
			w.scripts.Hear(mob, script.Foe{Name: msg.Player.Name(), IsPlayer: true}, cmd.Value)
		}
		return
	}
	msg.Fail(event.ToPlayerNotFound)
}
