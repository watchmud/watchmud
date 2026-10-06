package moon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// against the almanac: moments the moon was new or full
func TestPhaseAt_almanac(t *testing.T) {
	for when, want := range map[string]Phase{
		"2024-01-11T11:57:00Z": New,
		"2024-01-25T17:54:00Z": Full,
		"2025-10-07T03:47:00Z": Full,
		"2025-10-21T12:25:00Z": New,
		"2026-09-26T16:49:00Z": Full,
		"2026-10-10T15:50:00Z": New,
	} {
		at, err := time.Parse(time.RFC3339, when)
		assert.NoError(t, err)
		assert.Equal(t, want, PhaseAt(at), when)
	}
}

func TestIllumination(t *testing.T) {
	full, _ := time.Parse(time.RFC3339, "2024-01-25T17:54:00Z")
	newMoon, _ := time.Parse(time.RFC3339, "2024-01-11T11:57:00Z")
	assert.Greater(t, Illumination(full), 0.99)
	assert.Less(t, Illumination(newMoon), 0.01)
}

// the phases go round in order through a month
func TestPhaseAt_inOrder(t *testing.T) {
	start, _ := time.Parse(time.RFC3339, "2024-01-11T11:57:00Z")
	seen := []Phase{PhaseAt(start)}
	for h := 0; h < 29*24; h += 6 {
		if p := PhaseAt(start.Add(time.Duration(h) * time.Hour)); p != seen[len(seen)-1] {
			seen = append(seen, p)
		}
	}
	assert.Equal(t, []Phase{New, WaxingCrescent, FirstQuarter, WaxingGibbous, Full, WaningGibbous, LastQuarter, WaningCrescent, New}, seen)
}
