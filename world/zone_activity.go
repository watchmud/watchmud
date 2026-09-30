package world

import (
	"time"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/rules"
)

// DoZoneActivity for each zone based on time.Now()
// For example, zone resets.
func (w *World) DoZoneActivity() {
	w.doZoneActivity(time.Now())
}

// doZoneActivity for each zone based on pulse time
// For example, zone resets.
func (w *World) doZoneActivity(now time.Time) {
	for _, z := range w.content.Zones {
		if z.ResetMode == rules.ZoneResetNoPlayers || z.ResetMode == rules.ZoneResetAlways {
			// is it time yet for this zone's lifetime?
			if now.Sub(z.LastReset) > z.Lifetime {
				if errs := z.Reset(w.occupancy); len(errs) != 0 {
					for _, err := range errs {
						log.Warn().Str("zone", z.Id).Err(err).Msg("zone reset error")
					}
				}
			}
		}
	}
}
