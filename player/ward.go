package player

import "time"

// Ward is how much damage the player's ward will still take: zero once it's
// spent or its time is up. Expiry is read, not scheduled -- nothing has to
// run for a ward to fade.
func (p *Player) Ward(now time.Time) int {
	if !now.Before(p.wardUntil) {
		return 0
	}
	return p.ward
}

// SetWard puts a ward of amount on the player, from now for d. A second
// ward refreshes rather than stacks: the shield is the larger of what's left
// and what's new, and the clock starts again.
func (p *Player) SetWard(amount int, now time.Time, d time.Duration) {
	p.ward = max(p.Ward(now), amount)
	p.wardUntil = now.Add(d)
}

// AbsorbWard takes what it can of a blow's damage into the ward, and says
// how much it took and whether that used the ward up. Whatever is left of
// the damage is the caller's to deal.
func (p *Player) AbsorbWard(damage int, now time.Time) (absorbed int, broke bool) {
	left := p.Ward(now)
	if left == 0 || damage <= 0 {
		return 0, false
	}
	absorbed = min(left, damage)
	p.ward = left - absorbed
	return absorbed, p.ward == 0
}
