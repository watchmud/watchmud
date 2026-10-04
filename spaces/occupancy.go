package spaces

import (
	"uuid"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/ordered"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

// Occupancy is who is in which room, both directions, and the only code
// that writes a room's player and mob lists. The room's lists answer
// "who is here" in a stable order; the maps here are the index for
// "where is this one". Two copies are fine, two writers were the bug --
// see ROADMAP "Dual location bookkeeping".
type Occupancy struct {
	playerRoom map[*player.Player]*Room
	mobileRoom map[*mobile.Instance]*Room
	// mobiles is every mob in the world, in the order it was placed, because
	// mobileRoom is a map: ranging it for the mobile pulse had the mobs
	// wander, aggro and regen in a different order every time.
	mobiles *ordered.List[uuid.UUID, *mobile.Instance]
}

func NewOccupancy() *Occupancy {
	return &Occupancy{
		playerRoom: make(map[*player.Player]*Room),
		mobileRoom: make(map[*mobile.Instance]*Room),
		mobiles:    ordered.NewList[uuid.UUID]((*mobile.Instance).Id),
	}
}

// RoomOfPlayer is the room p is in, nil if they are nowhere.
func (o *Occupancy) RoomOfPlayer(p *player.Player) *Room {
	return o.playerRoom[p]
}

// PlacePlayer puts p in r without telling anyone: login and creation.
func (o *Occupancy) PlacePlayer(p *player.Player, r *Room) {
	if cur := o.playerRoom[p]; cur != nil {
		log.Error().Str("player", p.Name()).Str("room", cur.Location().String()).
			Msg("PlacePlayer: already placed")
		return
	}
	r.addPlayer(p)
	o.playerRoom[p] = r
}

// MovePlayer takes p out of wherever they are and into dest, telling both
// rooms. A player who is nowhere just arrives.
func (o *Occupancy) MovePlayer(p *player.Player, dir rules.Direction, dest *Room) {
	if src := o.playerRoom[p]; src != nil {
		src.playerLeaves(p, dir)
	}
	dest.playerEnters(p)
	o.playerRoom[p] = dest
}

// RemovePlayer takes p out of the world's rooms without telling anyone.
func (o *Occupancy) RemovePlayer(p *player.Player) {
	if r := o.playerRoom[p]; r != nil {
		r.removePlayer(p)
	}
	delete(o.playerRoom, p)
}

func (o *Occupancy) RoomOfMobile(mob *mobile.Instance) *Room {
	return o.mobileRoom[mob]
}

// PlaceMobile puts mob in r without telling anyone: zone resets and `load`.
func (o *Occupancy) PlaceMobile(mob *mobile.Instance, r *Room) {
	if cur := o.mobileRoom[mob]; cur != nil {
		log.Error().Str("mob", mob.Name()).Str("room", cur.Location().String()).
			Msg("PlaceMobile: already placed")
		return
	}
	if err := r.addMobile(mob); err != nil {
		log.Error().Err(err).Str("room", r.Location().String()).Msg("PlaceMobile")
		return
	}
	if err := o.mobiles.Add(mob); err != nil {
		log.Error().Err(err).Str("mob", mob.Name()).Msg("PlaceMobile")
	}
	o.mobileRoom[mob] = r
}

func (o *Occupancy) MoveMobile(mob *mobile.Instance, dir rules.Direction, dest *Room) {
	if src := o.mobileRoom[mob]; src != nil {
		src.mobileLeaves(mob, dir)
	}
	dest.mobileEnters(mob)
	o.mobileRoom[mob] = dest
}

func (o *Occupancy) RemoveMobile(mob *mobile.Instance) {
	if r := o.mobileRoom[mob]; r != nil {
		if err := r.removeMobile(mob); err != nil {
			log.Error().Err(err).Str("room", r.Location().String()).Msg("RemoveMobile")
		}
		if err := o.mobiles.Remove(mob); err != nil {
			log.Error().Err(err).Str("mob", mob.Name()).Msg("RemoveMobile")
		}
	}
	delete(o.mobileRoom, mob)
}

// Mobiles is every mob in the world, in the order they were placed; moving
// doesn't change it. A copy, since the mobile pulse moves and kills mobs
// while it walks the list.
func (o *Occupancy) Mobiles() []*mobile.Instance {
	return o.mobiles.Slice()
}

func (o *Occupancy) MobileCount(defId string) int {
	count := 0
	for mob := range o.mobiles.All() {
		// A summon is its summoner's, not the zone's: counting it would keep a
		// reset from refilling the room that it was called away from.
		if mob.Definition.Id == defId && mob.Summoner == nil {
			count++
		}
	}
	return count
}
