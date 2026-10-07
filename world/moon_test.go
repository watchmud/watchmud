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
	s.Require().True(s.w.fullMoonTonight())
	s.w.DoMobileActivity()
	s.Assert().False(s.w.fightLedger.InFight(s.p), "full, but daylight")

	s.at(FullMoonNight)
	s.w.DoMobileActivity()
	s.Assert().True(s.w.fightLedger.InFight(s.p))
}

// and with nobody there to go for, it roams: a moonstruck dog frozen in one
// room all night would be the safest thing out there
func (s *moonSuite) TestMoonstruck_roams() {
	s.w.RemovePlayer(s.p)
	s.mob.Definition.Wandering = rules.WanderDefinition{CanWander: true, Style: rules.WanderRandom}
	s.mob.LastWanderingTime = time.Time{}

	s.at(FullMoonNight)
	s.w.DoMobileActivity()
	s.Assert().False(s.mob.LastWanderingTime.IsZero(), "it went wandering")
}

// the night is one thing: full at its middle, full all through it, and the
// warning that morning agrees with what happens after dark
func (s *moonSuite) TestTonight() {
	pdt := func(day, hour int) time.Time { return time.Date(2026, 9, day, hour, 0, 0, 0, wrathrockTime) }
	s.Assert().Equal(pdt(27, 0), tonight(pdt(26, 9)), "in the morning, tonight is the coming night")
	s.Assert().Equal(pdt(27, 0), tonight(pdt(26, 22)))
	s.Assert().Equal(pdt(27, 0), tonight(pdt(27, 5)), "before dawn, it's the night under way")
	s.Assert().Equal(pdt(28, 0), tonight(pdt(27, 6)))

	// every hour of a full-moon night is one
	for h := 18; h < 30; h++ {
		s.at(pdt(26, 0).Add(time.Duration(h) * time.Hour))
		s.Assert().True(s.w.fullMoonNight(), "hour %d", h)
	}
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
