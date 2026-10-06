package world

import (
	"errors"
	"fmt"
	"iter"
	"maps"
	"slices"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/combat"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/script"
	"github.com/watchmud/watchmud/spaces"
)

// noinspection GoNameStartsWithPackageName
type World struct {
	StartRoom *spaces.Room
	VoidRoom  *spaces.Room
	DeathRoom *spaces.Room // where a player who dies wakes up
	content   *loader.Content

	roller rules.Roller
	store  player.Store

	playerList *player.List // list of players in world
	occupancy  *spaces.Occupancy

	fightLedger *combat.FightLedger
	// scripts runs the Lua mobs name; see world/scripts.go.
	scripts *script.Runtime

	// now is the clock cooldowns read; tests replace it rather than sleep.
	now func() time.Time
	// pace is how fast now runs against the wall clock; see SetPace.
	pace float64

	reservedNames map[string]bool // by player.NameKey; see IsReservedName

	groups *groups // who follows whom; see world/group.go
}

// New creates a brand-new World based on this content
func New(c *loader.Content, s player.Store, roller rules.Roller) (w *World, err error) {
	w = &World{
		content:     c,
		playerList:  player.NewList(),
		occupancy:   spaces.NewOccupancy(),
		fightLedger: combat.NewFightLedger(),
		groups:      newGroups(),
		now:         time.Now,
		pace:        1,
		roller:      roller,
		store:       s,
	}
	if w.scripts, err = script.NewRuntime(c.Scripts, liveRoller{w}, script.Actions{
		Say:     w.mobSays,
		Summon:  w.summon,
		Summons: w.liveSummons,
		Junk:    w.junkHere,
		Sweep:   w.sweep,
		Flee:    w.mobFlees,
		Take:    w.mobTakes,
	}); err != nil {
		return nil, fmt.Errorf("building world: %w", err)
	}
	if err := checkEffects(c.Catalog); err != nil {
		return nil, fmt.Errorf("building world: %w", err)
	}
	w.reservedNames = reservedNames(c)
	if err := w.initialLoad(); err != nil {
		return nil, fmt.Errorf("building world: %w", err)
	}
	log.Info().Msg("World built.")
	return w, nil
}

func (w *World) initialLoad() (err error) {
	content := w.content
	settings := content.Settings
	if w.StartRoom, err = content.Room(settings.Start.ZoneId, settings.Start.RoomId); err != nil {
		return fmt.Errorf("start room: %w", err)
	}
	if w.VoidRoom, err = content.Room(settings.Void.ZoneId, settings.Void.RoomId); err != nil {
		return fmt.Errorf("void room: %w", err)
	}
	if w.DeathRoom, err = content.Room(settings.PlayerDeath.ZoneId, settings.PlayerDeath.RoomId); err != nil {
		return fmt.Errorf("player-death room: %w", err)
	}

	// TODO replace
	// Process the zone commands that say which
	// mob and object instances to create and where. Distinct from building the
	// world, since this recurs throughout runtime.
	for _, zoneId := range slices.Sorted(maps.Keys(content.Zones)) {
		if errs := content.Zones[zoneId].Reset(w.occupancy); len(errs) > 0 {
			return fmt.Errorf("initial reset of zone %s: %w", zoneId, errors.Join(errs...))
		}
	}
	return nil
}

// ReturnPlayer puts a returning player back in the room their record says
// they were last in, or the start room if it doesn't say or that room no
// longer exists. A content edit that removes a room must not strand anybody,
// the same as the missing object definitions player.FromRecord forgives.
func (w *World) ReturnPlayer(p *player.Player, zoneId, roomId string) {
	r, found := w.findRoomById(zoneId, roomId)
	if !found {
		if zoneId != "" || roomId != "" {
			p.Log().Warn().Msgf("last room %s.%s not found, using the start room", zoneId, roomId)
		}
		r = w.StartRoom
	}
	w.PlacePlayer(p, r)
}

// Arrive finishes a login: the room hears who just appeared in it, and the
// player is shown where they are -- which, now that a returning player comes
// back to wherever they left, isn't necessarily where they expect.
//
// Separate from AddPlayer and ReturnPlayer so the transport's login events go
// out first; the player's own description has to land after the login
// conversation has ended, not in the middle of it.
//
// It is also where a wizard's nohassle goes on: every session starts with
// aggressive mobs leaving them alone, and anyone else with it off.
func (w *World) Arrive(p *player.Player) {
	w.arrive(p, false)
}

// ArriveNew is Arrive for a character just created: the room is told it's
// their first time, so anyone there -- a socialite bot, say -- knows to
// make them welcome.
func (w *World) ArriveNew(p *player.Player) {
	w.arrive(p, true)
}

func (w *World) arrive(p *player.Player, first bool) {
	p.SetNoHassle(p.IsWizard())
	r := w.playerRoom(p)
	r.SendExcept(p, event.EnteredGame{Actor: p.Name(), First: first})
	p.Send(event.Color{On: p.Color()}) // ahead of the first thing worth coloring
	p.Send(r.DescriptionExcept(p))
	w.backfill(p)
}

// Welcome tells a brand-new character where to go first. After Arrive, so it
// follows the description of the room it's talking about; creation only, so a
// returning player isn't told every time.
func (w *World) Welcome(p *player.Player) {
	if text := w.content.Settings.Welcome; text != "" {
		p.Send(event.Welcome{Text: text})
	}
}

func (w *World) PlacePlayer(p *player.Player, r *spaces.Room) {
	p.Log().Debug().Str("room", r.Location().String()).Msg("Adding player to world")
	w.playerList.Add(p)
	w.occupancy.PlacePlayer(p, r)
}

func (w *World) RemovePlayer(p *player.Player) {
	p.Log().Debug().Msg("Removing player")
	w.fightLedger.EndAllFightsWith(p.Id())
	w.leaveGroups(p)
	w.occupancy.RemovePlayer(p)
	w.playerList.Remove(p)
}

// MovePlayer from place to place. Player is moving from src room to dest room.
func (w *World) movePlayer(p *player.Player, dir rules.Direction, dest *spaces.Room) {
	w.occupancy.MovePlayer(p, dir, dest)
}

// movePlayerMagically from room they're in to somewhere else.
func (w *World) movePlayerMagically(p *player.Player, dest *spaces.Room) {
	w.movePlayer(p, rules.DirectionNone, dest)
}

// moveMobile to room
func (w *World) moveMobile(mob *mobile.Instance, dir rules.Direction, dest *spaces.Room) {
	w.occupancy.MoveMobile(mob, dir, dest)
}

// PlaceMobile adds a mobile instance to a room in the world.
func (w *World) PlaceMobile(mob *mobile.Instance, targetRoom *spaces.Room) {
	w.occupancy.PlaceMobile(mob, targetRoom)
}

// RemoveMobile removes a mobile instance from the world.
func (w *World) RemoveMobile(mob *mobile.Instance) {
	w.occupancy.RemoveMobile(mob)
	w.scripts.Forget(mob)
}

func (w *World) Mobiles() []*mobile.Instance {
	return w.occupancy.Mobiles()
}

// roleName resolves a player's equipment weights to a role's display name,
// empty when the gear adds up to no role at all. A role is never stored, so
// every caller that wants one derives it here -- see CLAUDE.md, "Lineage and
// Role".
func (w *World) roleName(weights map[string]int) string {
	if r := w.content.Catalog.RoleFor(weights); r != nil {
		return r.Name
	}
	return ""
}

// playerRoom returns the room a player is in, or VoidRoom if we can't figure that out.
// Does not return nil.
func (w *World) playerRoom(p *player.Player) *spaces.Room {
	r := w.occupancy.RoomOfPlayer(p)
	if r == nil {
		p.Log().Warn().Msg("player not in a room!")
		return w.VoidRoom
	}
	return r
}

// mobileRoom returns the room a mobile is in, or VoidRoom if we don't know.
// Does not return nil.
func (w *World) mobileRoom(mob *mobile.Instance) *spaces.Room {
	r := w.occupancy.RoomOfMobile(mob)
	if r == nil {
		return w.VoidRoom
	}
	return r
}

// roomOf is the room a combatant is standing in, nil if they are nowhere.
// Fights don't remember where they started: nobody can leave a fight without
// ending it, so where the fighter stands is where the fight is.
func (w *World) roomOf(c combat.Combatant) *spaces.Room {
	switch c := c.(type) {
	case *player.Player:
		return w.occupancy.RoomOfPlayer(c)
	case *mobile.Instance:
		return w.occupancy.RoomOfMobile(c)
	}
	return nil
}

// Find room by zone id and room id.
func (w *World) findRoomById(zoneId string, roomId string) (*spaces.Room, bool) {
	if z, zoneExists := w.content.Zones[zoneId]; zoneExists {
		if r, roomExists := z.Rooms[roomId]; roomExists {
			return r, true
		}
	}
	return nil, false
}

func (w *World) findRoomByLocation(loc *player.Location) (*spaces.Room, bool) {
	if loc == nil {
		return nil, false
	}
	return w.findRoomById(loc.ZoneId, loc.RoomId)
}

// IsPlaying says whether a character by that name is in the world right now.
func (w *World) IsPlaying(name string) bool {
	return w.findPlayerByName(name) != nil
}

func (w *World) findPlayerByName(name string) *player.Player {
	return w.playerList.FindByName(name)
}

// Players is everyone in the world.
func (w *World) Players() iter.Seq[*player.Player] {
	return w.playerList.All()
}

// SendToAllPlayers send a message to all players in the world.
func (w *World) SendToAllPlayers(message interface{}) {
	for p := range w.playerList.All() {
		p.Send(message)
	}
}

// SendToAllPlayersExcept send a message to all players in the world except the exception player.
func (w *World) SendToAllPlayersExcept(exception *player.Player, message interface{}) {
	for p := range w.playerList.AllExcept(exception) {
		p.Send(message)
	}
}

func (w *World) Zones() iter.Seq[*spaces.Zone] {
	return maps.Values(w.content.Zones)
}

func (w *World) Zone(zoneId string) *spaces.Zone {
	return w.content.Zones[zoneId]
}

// ObjectDefinition looks up by zoneId and definitionId, implementing player.DefinitionSource
func (w *World) ObjectDefinition(zoneId, definitionId string) (*object.Definition, bool) {
	z := w.Zone(zoneId)
	if z == nil {
		return nil, false
	}
	d, ok := z.ObjectDefinitions[definitionId]
	if !ok {
		return nil, false
	}
	return d, true
}

// SetPace runs the world's clock -- what cooldowns read -- at rate times the
// wall clock. The server sets it from its tick, so a test world ticking a
// hundred times fast has cooldowns a hundred times short, the same as
// everything else in it. 1 is real time.
func (w *World) SetPace(rate float64) {
	start := time.Now()
	w.pace = rate
	w.now = func() time.Time {
		return start.Add(time.Duration(float64(time.Since(start)) * rate))
	}
}

// Pace is what SetPace last set: 1 unless something sped the world up.
func (w *World) Pace() float64 { return w.pace }
