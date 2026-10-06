package world

import (
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // the deploy image is distroless: no zoneinfo of its own

	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/moon"
	"github.com/watchmud/watchmud/player"
)

// Wrathrock keeps Seattle's time: the game's clock is the Pacific
// Northwest's, daylight saving and all.
var wrathrockTime = mustLocation("America/Los_Angeles")

func mustLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err) // embedded by time/tzdata: can't be missing
	}
	return loc
}

// handleSplit shares coins among the group in the room, the splitter
// included; what doesn't divide evenly stays with the splitter.
func (w *World) handleSplit(msg *gameserver.HandlerParameter, cmd command.Split) {
	n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(cmd.Amount), " coins"))
	if err != nil || n <= 0 {
		msg.Fail(event.NoValue)
		return
	}
	room := w.playerRoom(msg.Player)
	var here []*player.Player
	for _, p := range w.groups.members(msg.Player) {
		if p != msg.Player && w.playerRoom(p) == room {
			here = append(here, p)
		}
	}
	if len(here) == 0 {
		msg.Fail(event.NoOneToSplit)
		return
	}
	if n > msg.Player.Coins() {
		msg.Fail(event.NotEnoughCoins)
		return
	}
	among := len(here) + 1
	each := n / among
	if each == 0 {
		msg.Fail(event.NotEnoughCoins)
		return
	}
	msg.Player.Spend(each * len(here))
	for _, p := range here {
		p.AddCoins(each)
	}
	// only to those who got some: a bystander would read "you get"
	split := event.SplitCoins{Actor: msg.Player.Name(), Each: each, Among: among}
	msg.Player.Send(split)
	for _, p := range here {
		p.Send(split)
	}
}

// handleWhere lists the players in the same zone, and the room each is in.
func (w *World) handleWhere(msg *gameserver.HandlerParameter, cmd command.Where) {
	zone := w.playerRoom(msg.Player).Zone
	list := event.WhereList{Zone: zone.Name}
	for p := range w.playerList.All() {
		if r := w.playerRoom(p); r != nil && r.Zone == zone {
			list.Players = append(list.Players, event.WhereEntry{Name: p.Name(), Room: r.Name})
		}
	}
	msg.Player.Send(list)
}

func (w *World) handleCommands(msg *gameserver.HandlerParameter, cmd command.Commands) {
	msg.Player.Send(event.CommandList{})
}

// handleTime is Wrathrock's clock, which is Seattle's.
func (w *World) handleTime(msg *gameserver.HandlerParameter, cmd command.Time) {
	now := w.now().In(wrathrockTime)
	msg.Player.Send(event.TimeOfDay{Clock: now.Format("3:04 pm"), Day: now.Format("Monday, January 2"),
		Part: partOfDay(now.Hour()), Moon: string(moon.PhaseAt(w.moonTime()))})
}

func partOfDay(hour int) string {
	switch {
	case hour >= 5 && hour < 12:
		return "morning"
	case hour >= 12 && hour < 17:
		return "afternoon"
	case hour >= 17 && hour < 21:
		return "evening"
	}
	return "night"
}

// handleWimpy sets, or says, the health under which a player flees on their
// own. Kept on the record.
func (w *World) handleWimpy(msg *gameserver.HandlerParameter, cmd command.Wimpy) {
	if cmd.Amount == "" {
		msg.Player.Send(event.WimpySet{At: msg.Player.Wimpy()})
		return
	}
	n, err := strconv.Atoi(strings.TrimSpace(cmd.Amount))
	if err != nil || n < 0 || n >= msg.Player.MaxHealth() {
		msg.Fail(event.BadRequest)
		return
	}
	msg.Player.SetWimpy(n)
	msg.Player.Send(event.WimpySet{At: n, Changed: true})
}
