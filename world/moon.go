package world

import (
	"time"

	"github.com/watchmud/watchmud/moon"
)

// SetMoonClock is the clock the moon reads, apart from the world's: tests pin
// it, so a full moon never changes what a test sees once a month. Nil is the
// world's own clock.
func (w *World) SetMoonClock(now func() time.Time) { w.moonNow = now }

func (w *World) moonTime() time.Time {
	if w.moonNow != nil {
		return w.moonNow()
	}
	return w.now()
}

// The moon over Wrathrock is the one over Seattle: its phase from the date,
// night from Seattle's clock (wrathrockTime). A full moon weighs on a few
// things -- placeholders, like every number, and meant to be noticed:
//
//   - a "moonstruck" mob turns aggressive on a full-moon night (the
//     Hollowfields' wild dogs);
//   - loot's chance of a power bump doubles on a full-moon night;
//   - on a full-moon day, anyone arriving is warned.

// fullMoon is whether the moon is full now, day or night.
func (w *World) fullMoon() bool {
	return moon.PhaseAt(w.moonTime()) == moon.Full
}

// fullMoonNight is a full moon and dark in Seattle: 6 pm to 6 am.
func (w *World) fullMoonNight() bool {
	h := w.moonTime().In(wrathrockTime).Hour()
	return w.fullMoon() && (h >= 18 || h < 6)
}

// NewMoon and FullMoonNight are moments to pin the moon at: a new moon, and a
// full moon at 10 pm in Seattle. For tests, here and in other packages.
var (
	NewMoon       = time.Date(2026, 10, 10, 15, 50, 0, 0, time.UTC)
	FullMoonNight = time.Date(2026, 9, 27, 5, 0, 0, 0, time.UTC)
)
