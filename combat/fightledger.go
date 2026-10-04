package combat

import (
	"fmt"
	"uuid"
)

type FightLedger struct {
	fightMap map[uuid.UUID]*Fight
	nextSeq  uint64

	// stuns is swings left to skip, by combatant rather than by fight, so a
	// stunned mob that is provoked or retargeted stays stunned.
	stuns map[uuid.UUID]int
}

func NewFightLedger() *FightLedger {
	return &FightLedger{
		fightMap: make(map[uuid.UUID]*Fight),
		stuns:    make(map[uuid.UUID]int),
	}
}

func (f *FightLedger) Fight(fighter, fightee Combatant) error {
	if f.IsFighting(fighter) {
		// Callers check first -- kill answers AlreadyFighting, aggro skips a
		// mob that's busy -- so reaching this is a bug, not a player's mistake.
		return fmt.Errorf("fighter is already fighting someone")
	}
	f.fightMap[fighter.Id()] = f.newFight(fighter, fightee)

	if !f.IsFighting(fightee) {
		f.fightMap[fightee.Id()] = f.newFight(fightee, fighter)
	}
	return nil
}

// Turn points fighter at target instead of whoever it was fighting, starting a
// one-way fight if it had none. It is the only thing that overrides "whoever
// engaged first" -- a provoke. The fighter keeps its pulse, so being turned
// is never a free swing.
func (f *FightLedger) Turn(fighter, target Combatant) {
	turned := f.newFight(fighter, target)
	if old, ok := f.fightMap[fighter.Id()]; ok {
		turned.LastPulse = old.LastPulse
	}
	f.fightMap[fighter.Id()] = turned
}

// Stun makes c skip its next `rounds` swings. A stun refreshes rather than
// stacks -- the longer of what's left and what's new -- so two players taking
// turns can't hold a mob still forever.
func (f *FightLedger) Stun(c Combatant, rounds int) {
	f.stuns[c.Id()] = max(f.stuns[c.Id()], rounds)
}

// Stunned is how many swings c has left to skip.
func (f *FightLedger) Stunned(c Combatant) int {
	return f.stuns[c.Id()]
}

// SpendStun uses up one stunned round, answering whether c had one to spend:
// true means c doesn't swing this round.
func (f *FightLedger) SpendStun(c Combatant) bool {
	left, stunned := f.stuns[c.Id()]
	if !stunned {
		return false
	}
	if left <= 1 {
		delete(f.stuns, c.Id())
	} else {
		f.stuns[c.Id()] = left - 1
	}
	return true
}

func (f *FightLedger) newFight(fighter, fightee Combatant) *Fight {
	f.nextSeq++
	return newFight(fighter, fightee, f.nextSeq)
}

func (f *FightLedger) InFight(c Combatant) bool {
	return f.isBeingFought(c) || f.IsFighting(c)
}

func (f *FightLedger) IsFighting(c Combatant) bool {
	_, exists := f.fightMap[c.Id()]
	return exists
}

func (f *FightLedger) isBeingFought(c Combatant) bool {
	for _, fight := range f.fightMap {
		if fight.Fightee == c {
			return true
		}
	}
	return false
}

func (f *FightLedger) GetFight(fighter Combatant) *Fight {
	return f.fightMap[fighter.Id()]
}

func (f *FightLedger) GetFights() (result []*Fight) {
	for _, v := range f.fightMap {
		result = append(result, v)
	}
	return result
}

func (f *FightLedger) EndFight(fighter Combatant) {
	delete(f.fightMap, fighter.Id())
}

// EndAllFightsWith takes someone out of every fight, both ways -- they died,
// fled or left -- ends any stun on them, and then retargets anyone that leaves
// still under attack.
func (f *FightLedger) EndAllFightsWith(id uuid.UUID) {
	for k, v := range f.fightMap {
		if v.Fighter.Id() == id || v.Fightee.Id() == id {
			delete(f.fightMap, k)
		}
	}
	delete(f.stuns, id)
	f.retarget()
}

// retarget gives anyone who is being fought, but has stopped fighting, a fight
// back against the attacker who has been at it longest.
//
// Without it the King kills the tank and then stands there: still "in a
// fight", so aggro skips him, and with no fight of his own, so violence never
// has him swing. Earliest is the rule that held him on the tank in the first
// place -- Fight never overwrites a target -- carried on to whoever is next.
//
// A new fight is against someone already fighting, so it can't leave anyone
// else stranded; one pass is enough.
func (f *FightLedger) retarget() {
	earliest := make(map[uuid.UUID]*Fight)
	for _, fight := range f.fightMap {
		target := fight.Fightee.Id()
		if f.IsFighting(fight.Fightee) {
			continue
		}
		if prev, ok := earliest[target]; !ok || fight.seq < prev.seq {
			earliest[target] = fight
		}
	}
	for target, attack := range earliest {
		f.fightMap[target] = f.newFight(attack.Fightee, attack.Fighter)
	}
}
