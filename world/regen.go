package world

import "github.com/watchmud/watchmud/rules"

// Regenerate gives everyone who isn't fighting a little health back:
// players and mobs both, or a boss could be worn down by dying at it
// over and over.
//
// Not while fighting, on either side of the fight. Healing mid-fight
// turns a slow fight into one that never ends. And never the dead:
// regen is not resurrection.
//
// Mana is the exception: players get it back fighting or not, since a
// healer with nothing left to cast is a spectator.
//
// Sitting, resting and sleeping bring both back faster
// (player.Position.RegenPercent): the reason to sit down at all.
func (w *World) Regenerate() {
	for p := range w.Players() {
		// mana before the fight check: a healer gets it back mid-fight
		faster := p.Position().RegenPercent()
		p.RestoreMana(rules.ManaRegenAmount(p.MaxMana()) * faster / 100)
		if p.Dead() || w.fightLedger.InFight(p) {
			continue
		}
		p.RestoreHealth(rules.RegenAmount(p.MaxHealth()) * faster / 100)
	}
	for _, mob := range w.Mobiles() {
		if mob.Dead() || w.fightLedger.InFight(mob) {
			continue
		}
		mob.RestoreHealth(rules.RegenAmount(mob.Definition.MaxHealth))
	}
}
