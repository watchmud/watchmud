package world

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/rules"
)

type moonSuite struct {
	worldTestSuite
	mob *mobile.Instance
}

func TestMoonSuite(t *testing.T) { suite.Run(t, new(moonSuite)) }

func (s *moonSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	var found bool
	s.mob, found = s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	s.mob.Definition.SetFlag(rules.MobileFlagMoonstruck)
}

func (s *moonSuite) at(t time.Time) { s.w.SetMoonClock(func() time.Time { return t }) }

// a moonstruck mob leaves you be, until a full-moon night
func (s *moonSuite) TestMoonstruck() {
	s.at(NewMoon)
	s.w.DoMobileActivity()
	s.Assert().False(s.w.fightLedger.InFight(s.p))

	s.at(FullMoonNight.Add(-12 * time.Hour)) // full, but the middle of the day
	s.Require().True(s.w.fullMoon())
	s.w.DoMobileActivity()
	s.Assert().False(s.w.fightLedger.InFight(s.p), "full, but daylight")

	s.at(FullMoonNight)
	s.w.DoMobileActivity()
	s.Assert().True(s.w.fightLedger.InFight(s.p))
}

// anyone arriving on a full-moon day is warned
func (s *moonSuite) TestWarning() {
	s.at(FullMoonNight)
	s.w.Arrive(s.p)
	s.Assert().GreaterOrEqual(indexOf[event.MoonWarning](s.r), 0)

	s.r.Clear()
	s.at(NewMoon)
	s.w.Arrive(s.p)
	s.Assert().Equal(-1, indexOf[event.MoonWarning](s.r))
}
