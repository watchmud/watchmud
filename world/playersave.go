package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/player"
)

// QueuePlayerRecords hands a record of everyone in the world to the store.
// This is the timed save, and the last one at shutdown: whatever changed a
// player -- a command, a fight, a death -- is in the next one.
func (w *World) QueuePlayerRecords() {
	for p := range w.Players() {
		if err := w.store.Save(w.record(p)); err != nil {
			log.Error().Err(err).Str("player", p.Name()).Msg("queueing player save")
		}
	}
}

// saveHandedOver saves, now rather than on the timer, a player who may just
// have let something go where someone else can take it -- dropped, given,
// put in a chest, donated, split -- and everyone in their room, which is
// where a give or a split lands. Otherwise a crash before the timed save
// brings the giver back still holding what the taker, saved on quitting,
// has as well. A save is only queued (writebehind), and one that changes
// nothing is skipped, so a refused command costs nothing.
func (w *World) saveHandedOver(p *player.Player) {
	room := w.playerRoom(p)
	if room == nil {
		return
	}
	for _, other := range room.Players() {
		if err := w.store.Save(w.record(other)); err != nil {
			log.Error().Err(err).Str("player", other.Name()).Msg("saving after a hand-over")
		}
	}
}

// Record is the player's record plus where they are standing. The player
// can't fill that in themselves: location lives in the world's Occupancy, not on the
// player. Every save goes through here, or the room is forgotten.
func (w *World) record(p *player.Player) *player.Record {
	rec := p.Record()
	// don't use version that returns Void if they aren't in a room,
	// we want to test for that case.
	if r := w.occupancy.RoomOfPlayer(p); r != nil {
		rec.LastZoneId = r.Zone.Id
		rec.LastRoomId = r.Id
	}
	return rec
}
