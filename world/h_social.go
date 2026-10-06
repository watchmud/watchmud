package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/player"
)

// handleSocial is a gesture from rules/socials.json, alone or at a player
// or mob in the room. A name that isn't a social is an unknown request: the
// parser sends every verb it doesn't know here.
func (w *World) handleSocial(msg *gameserver.HandlerParameter, cmd command.Social) {
	social := w.content.Catalog.Socials[cmd.Name]
	if social == nil {
		msg.Fail(event.UnknownCommand)
		return
	}
	me := msg.Player.Name()
	room := w.playerRoom(msg.Player)
	if cmd.Target == "" || player.NameKey(cmd.Target) == player.NameKey(me) {
		l := social.Alone
		room.Send(event.Socialized{Actor: me, ToActor: l.Fill(l.Self, me, ""), ToRoom: l.Fill(l.Room, me, "")})
		return
	}
	if social.At == nil {
		msg.Fail(event.SocialAlone)
		return
	}
	var target string
	if p, found := room.FindPlayer(cmd.Target); found {
		target = p.Name()
	} else if mob, found := room.FindMobile(cmd.Target); found {
		target = "the " + mob.Name()
	} else {
		msg.Fail(event.TargetNotFound)
		return
	}
	l := *social.At
	room.Send(event.Socialized{
		Actor:    me,
		Target:   target,
		ToActor:  l.Fill(l.Self, me, target),
		ToTarget: l.Fill(l.Victim, me, target),
		ToRoom:   l.Fill(l.Room, me, target),
	})
}

func (w *World) handleSocials(msg *gameserver.HandlerParameter, cmd command.Socials) {
	var names []string
	for _, s := range w.content.Catalog.SocialList() {
		names = append(names, s.Name)
	}
	msg.Player.Send(event.SocialList{Names: names})
}

// handleEmote: the room reads the player's name and then whatever they wrote.
func (w *World) handleEmote(msg *gameserver.HandlerParameter, cmd command.Emote) {
	if cmd.Text == "" {
		msg.Fail(event.NoValue)
		return
	}
	w.playerRoom(msg.Player).Send(event.Emoted{Actor: msg.Player.Name(), Text: cmd.Text})
}
