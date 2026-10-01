package object

import "github.com/watchmud/watchmud/rules"

// Durability is per-instance, because it is the whole point: two chain shirts
// off the same definition are the same kind of thing, and the one you have
// been wearing through fights is not in the same state as the one on the
// shelf. It is the first mutable thing an Instance has ever had.
//
// Broken is not destroyed. A broken piece stays worn and stays in inventory;
// what it stops doing is counting -- no armor class, no argument for any role
// -- so a breastplate giving out mid-fight is something the player sees
// happen to their AC and, if it was carrying them, to their role. That leaves
// something for a repair to act on later, which "we deleted it" would not.

// WearsOut is whether this thing has durability at all. Gear whose definition
// never got a max -- everything, before content grows a durability.json --
// never breaks and never has to be checked for it.
func (i *Instance) WearsOut() bool {
	return i.Definition.MaxDurability > rules.Indestructible
}

// Broken gear is worn, carried, and worth nothing.
func (i *Instance) Broken() bool {
	return i.WearsOut() && i.Durability <= 0
}

// Damage takes points off, reporting whether this is the blow that broke it
// -- true exactly once, so the caller can say so without having to remember
// what the state was beforehand. Damage to something already broken, or to
// something that doesn't wear out, does nothing.
func (i *Instance) Damage(points int) (justBroke bool) {
	if !i.WearsOut() || points <= 0 || i.Broken() {
		return false
	}
	i.Durability = max(i.Durability-points, 0)
	return i.Broken()
}

// Repair puts it back to new: the smithy's repair command, and the reason
// "broken, not destroyed" was a meaningful choice.
func (i *Instance) Repair() {
	i.Durability = i.Definition.MaxDurability
}
