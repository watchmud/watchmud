package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/player"
)

func (w *World) handleTell(msg *gameserver.HandlerParameter, cmd command.Tell) {
	receiver := w.findPlayerByName(cmd.To)
	if receiver == nil {
		msg.Fail(event.ToPlayerNotFound)
		return
	}
	w.tell(msg, receiver, cmd.Value)
}

// handleReply tells whoever last told this player something.
func (w *World) handleReply(msg *gameserver.HandlerParameter, cmd command.Reply) {
	name := w.lastTeller[msg.Player]
	if name == "" {
		msg.Fail(event.NoOneToReply)
		return
	}
	receiver := w.findPlayerByName(name)
	if receiver == nil {
		msg.Fail(event.ToPlayerNotFound)
		return
	}
	w.tell(msg, receiver, cmd.Value)
}

// tell is a tell or a reply, once its receiver is found: refused if either
// end has tells switched off, and remembered so the receiver can reply.
func (w *World) tell(msg *gameserver.HandlerParameter, receiver *player.Player, value string) {
	if value == "" {
		msg.Fail(event.NoValue)
		return
	}
	if !msg.Player.Tells() {
		msg.Fail(event.YourTellsOff)
		return
	}
	if !receiver.Tells() {
		msg.Fail(event.TellsOff)
		return
	}
	w.lastTeller[receiver] = msg.Player.Name()
	// one event, both ends of the conversation
	told := event.Told{
		From:  msg.Player.Name(),
		To:    receiver.Name(),
		Value: value,
	}
	receiver.Send(told)
	msg.Player.Send(told)
}
