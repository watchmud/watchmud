package world

import (
	"sort"

	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

func (w *World) handleWho(msg *gameserver.HandlerParameter, cmd command.Who) {
	// in the future we'll need to split this up by
	// rank, security, other things, but for now show
	// everybody everything.

	// playerName, lineage, role, (level and other things we don't have yet),
	// zoneName, roomName. The role is read off their gear as the list is
	// built, so it is current as of this moment and not a moment earlier.
	entries := []event.WhoEntry{}
	for p := range w.playerList.All() {
		r := w.playerRoom(p)
		entries = append(entries, event.WhoEntry{
			PlayerName: p.Name(),
			Bot:        p.IsBot(),
			Lineage:    p.LineageName(),
			Role:       w.roleName(p.RoleWeights()),
			ZoneName:   r.Zone.Name,
			RoomName:   r.Name,
		})
	}
	// sort results by name
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].PlayerName < entries[j].PlayerName
	})

	msg.Player.Send(event.Who{Players: entries})
}
