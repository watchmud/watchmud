// Package moon is the moon's phase, worked out from the date: the same
// everywhere on Earth at one instant. Where it's seen from -- the Pacific
// Northwest, for Wrathrock -- decides only whether it's night there to see it.
// A mean synodic month from a known new moon: good to within a few hours,
// which is all a game needs.
package moon

import (
	"math"
	"time"
)

// SynodicMonth is new moon to new moon, on average.
const SynodicMonth = time.Duration(29.530588853 * 24 * float64(time.Hour))

// knownNew is the new moon of 6 January 2000, 18:14 UTC.
var knownNew = time.Date(2000, 1, 6, 18, 14, 0, 0, time.UTC)

// Age is where in its month the moon is at t: 0 new, 0.5 full, back to 1.
func Age(t time.Time) float64 {
	a := math.Mod(float64(t.Sub(knownNew))/float64(SynodicMonth), 1)
	if a < 0 {
		a++
	}
	return a
}

// Illumination is the lit fraction of the disc, 0 to 1.
func Illumination(t time.Time) float64 {
	return (1 - math.Cos(2*math.Pi*Age(t))) / 2
}

// Phase is the moon's phase by name.
type Phase string

const (
	New            Phase = "new"
	WaxingCrescent Phase = "waxing crescent"
	FirstQuarter   Phase = "first quarter"
	WaxingGibbous  Phase = "waxing gibbous"
	Full           Phase = "full"
	WaningGibbous  Phase = "waning gibbous"
	LastQuarter    Phase = "last quarter"
	WaningCrescent Phase = "waning crescent"
)

// PhaseAt names the phase at t: the four principal phases each own about a
// day and three quarters either side of their moment, the rest between.
func PhaseAt(t time.Time) Phase {
	a := Age(t)
	const near = 1.75 / 29.530588853 // a principal phase's half-width, as age
	switch {
	case a < near || a > 1-near:
		return New
	case math.Abs(a-0.25) < near:
		return FirstQuarter
	case math.Abs(a-0.5) < near:
		return Full
	case math.Abs(a-0.75) < near:
		return LastQuarter
	case a < 0.25:
		return WaxingCrescent
	case a < 0.5:
		return WaxingGibbous
	case a < 0.75:
		return WaningGibbous
	}
	return WaningCrescent
}
