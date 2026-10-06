package world

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/rules"
)

type zoneActivitySuite struct{ worldTestSuite }

func TestZoneActivitySuite(t *testing.T) { suite.Run(t, new(zoneActivitySuite)) }

// a noPlayers zone waits for its players to leave; an always one doesn't
func (s *zoneActivitySuite) TestNoPlayersWaits() {
	z := s.w.StartRoom.Zone
	s.Require().NotEmpty(s.w.StartRoom.Players())
	now := time.Now()
	for _, mode := range []rules.ZoneReset{rules.ZoneResetNoPlayers, rules.ZoneResetAlways} {
		z.ResetMode = mode
		z.LastReset = now.Add(-z.Lifetime - time.Minute)
		s.w.doZoneActivity(now)
		if mode == rules.ZoneResetNoPlayers {
			s.Assert().True(z.LastReset.Before(now), "not with a player in it")
		} else {
			s.Assert().False(z.LastReset.Before(now), "always is always")
		}
	}

	z.ResetMode = rules.ZoneResetNoPlayers
	z.LastReset = now.Add(-z.Lifetime - time.Minute)
	s.w.RemovePlayer(s.p)
	s.w.doZoneActivity(now)
	s.Assert().False(z.LastReset.Before(now), "empty now")
}
