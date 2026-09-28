package combat

import (
	"fmt"
	"uuid"
)

type FightLedger struct {
	fightMap map[uuid.UUID]*Fight
	nextSeq  uint64
}

func NewFightLedger() *FightLedger {
	return &FightLedger{
		fightMap: make(map[uuid.UUID]*Fight),
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
// fled or left -- and then retargets anyone that leaves still under attack.
func (f *FightLedger) EndAllFightsWith(id uuid.UUID) {
	for k, v := range f.fightMap {
		if v.Fighter.Id() == id || v.Fightee.Id() == id {
			delete(f.fightMap, k)
		}
	}
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
