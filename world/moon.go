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
//   - on the day before a full-moon night, and during it, anyone arriving
//     is warned.

// A night is 6 pm to 6 am in Seattle, and whether it's a full-moon night is
// the moon's phase at its middle, midnight: a property of the whole night,
// so the dogs don't calm down at 11 pm because the phase ticked over, and
// "full moon tonight" said at breakfast is still true after dark.

// tonight is the midnight in the middle of tonight -- the night under way
// before 6 am, the coming one after.
func tonight(t time.Time) time.Time {
	t = t.In(wrathrockTime)
	day := t
	if t.Hour() >= 6 {
		day = t.AddDate(0, 0, 1)
	}
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, wrathrockTime)
}

// fullMoonTonight is whether tonight -- see tonight -- is a full-moon night.
func (w *World) fullMoonTonight() bool {
	return moon.PhaseAt(tonight(w.moonTime())) == moon.Full
}

// fullMoonNight is a full-moon night, and dark now.
func (w *World) fullMoonNight() bool {
	h := w.moonTime().In(wrathrockTime).Hour()
	return (h >= 18 || h < 6) && w.fullMoonTonight()
}

// NewMoon and FullMoonNight are moments to pin the moon at: a new moon, and a
// full moon at 10 pm in Seattle. For tests, here and in other packages.
var (
	NewMoon       = time.Date(2026, 10, 10, 15, 50, 0, 0, time.UTC)
	FullMoonNight = time.Date(2026, 9, 27, 5, 0, 0, 0, time.UTC)
)
