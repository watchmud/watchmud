package world

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
)

// DoMobileActivity and walk through all the mob instances that are
// in this world right now and tell them all to do the things that they
// should do at this time.
func (w *World) DoMobileActivity() {
	// for each mob in the world
	// wake it up and tell it to do stuff
	// don't limit this to per zone or per room or something
	// remember that mobs can leave the zone they started out in
	// if programmed to
	// or if they really want to...
	for _, mob := range w.occupancy.Mobiles() {
		if w.fightLedger.InFight(mob) {
			// actions where the mob is in a fight somewhere
		} else {
			// actions where the mob is NOT in a fight.

			moonstruck := mob.Definition.HasFlag(rules.MobileFlagMoonstruck) && w.fullMoonNight()
			if mob.Definition.HasFlag(rules.MobileFlagAggressive) || moonstruck {
				// a moonstruck mob still roams, hunting: frozen in one room
				// all night it would be the safest thing out there
				if !w.doMobAggro(mob) && moonstruck && mob.CanWander() {
					w.doMobWander(mob)
				}
			} else if mob.CanWander() {
				w.doMobWander(mob)
			}
		}
	}
}

// doMobAggro attacks the first player in the room it can see. A wizard with
// nohassle on isn't one of them, and doesn't shield whoever is standing next
// to them: the mob goes for the next player instead.
// It answers whether there was anyone to go for.
func (w *World) doMobAggro(mob *mobile.Instance) bool {
	room := w.mobileRoom(mob)
	if room.Flag(rules.RoomFlagNoFight) {
		return false // nobody fights here, whoever starts it
	}
	for _, p := range room.Players() {
		if p.IsWizard() && p.NoHassle() {
			continue
		}
		if err := w.startFight(mob, p); err != nil {
			log.Warn().Msgf("World.doMobAggro: %s error starting fight: %s", mob.Definition.Id, err)
		}
		return true
	}
	return false
}

func (w *World) doMobWander(mob *mobile.Instance) {
	switch mob.Definition.Wandering.Style {
	case rules.WanderRandom:
		// do random wander within the zone
		if err := w.doMobRandomWander(mob); err != nil {
			log.Warn().Msgf("World.DoMobileActivity: %s error randomly wandering: %s", mob.Definition.Id, err)
		}
	case rules.WanderFollowPath:
		if err := w.doMobFollowPathWander(mob); err != nil {
			log.Warn().Msgf("World.DoMobileActivity: %s error following path: %s", mob.Definition.Id, err)
		}
	default:
		// unknown or unhandled wandering style, do nothing.
	}
}

// pick a direction that is within the mob's zone and walk to it, if possible.
func (w *World) doMobRandomWander(mob *mobile.Instance) error {
	mob.LastWanderingTime = time.Now()
	mobRoom := w.mobileRoom(mob)

	// test wandering percentage
	if mob.CheckWanderChance() {
		dir := mobRoom.PickRandomDirection(true)
		if dir == rules.DirectionNone {
			return errors.New(fmt.Sprintf("Mobile ID '%s' is in a room without exit and can't wander out of it.", mob.Definition.Id))
		}
		w.moveMobile(mob, dir, mobRoom.DestinationRoom(dir))
		w.scripts.Arrive(mob)
	}
	return nil
}

func (w *World) doMobFollowPathWander(mob *mobile.Instance) error {
	mob.LastWanderingTime = time.Now()
	mobRoom := w.mobileRoom(mob)
	if mob.CheckWanderChance() {
		dir, changeDirection, err := getNextDirectionOnPath(mob, mobRoom)
		if err != nil {
			return err
		}
		if dir == rules.DirectionNone {
			return errors.New(fmt.Sprintf("doMobFollowPathWander: mobile ID '%s' can't figure out next place to go to (current '%s', path '%s')",
				mob.Definition.Id, mobRoom.Id, mob.Definition.Wandering.Path))
		}
		if !mobRoom.Passable(dir) {
			return nil // a closed door on its path: it waits
		}
		if changeDirection {
			mob.WanderingForward = !mob.WanderingForward
		}
		w.moveMobile(mob, dir, mobRoom.DestinationRoom(dir))
		w.scripts.Arrive(mob)
	}
	return nil
}

// Determine what direction this mob should travel next to stay on its path.
// Takes mob.WanderingForward into account and will reverse index at the path boundaries,
// returning changeDirection=true in that case, but WILL NOT update the state or
// modify the mob or room instances in any way.
func getNextDirectionOnPath(mob *mobile.Instance, mobRoom *spaces.Room) (dir rules.Direction, changeDirection bool, err error) {
	currentIndex, err := mob.GetIndexOnPath(mobRoom.Id)
	if err != nil {
		return rules.DirectionNone, false, err
	}
	nextIndex := -1

	if len(mob.Definition.Wandering.Path) < 2 {
		// the loader refuses one; this keeps a bad one from indexing -1
		return rules.DirectionNone, false, errors.New("a path needs two rooms")
	}
	if currentIndex < 0 {
		// note: this might be OK (if the mob was pulled off the path for some reason?)
		// TODO should it change to a random walk? or just wait here, or?
		return rules.DirectionNone, false, errors.New(fmt.Sprintf("Couldn't find current room %s in wander path %s", mobRoom.Id, mob.Definition.Wandering.Path))
	}
	if mob.WanderingForward {
		nextIndex = currentIndex + 1
		// is it time to change directions?
		if nextIndex > len(mob.Definition.Wandering.Path)-1 {
			// yep
			changeDirection = true
			nextIndex = currentIndex - 1
		}
	} else {
		nextIndex = currentIndex - 1
		// is it time to change directions?
		if nextIndex < 0 {
			// yep
			changeDirection = true
			nextIndex = currentIndex + 1
		}
	}
	roomToFind := mob.Definition.Wandering.Path[nextIndex]
	for _, rexit := range mobRoom.Exits(false) {
		if rexit.Room.Id == roomToFind {
			dir = rexit.Direction
			break
		}
	}
	if dir == rules.DirectionNone {
		return rules.DirectionNone, false, errors.New(fmt.Sprintf("Couldn't find destination room %s from current room exits %v", roomToFind, mobRoom.Exits(false)))
	}
	return
}
