package world

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type regenSuite struct {
	worldTestSuite
}

func TestRegenSuite(t *testing.T) {
	suite.Run(t, new(regenSuite))
}

func (s *regenSuite) TestHealsTheHurt() {
	s.p.TakeMeleeDamage(50)
	s.w.Regenerate()
	s.Assert().Equal(55, s.p.CurrentHealth(), "5% of 100")
}

func (s *regenSuite) TestStopsAtFull() {
	s.p.TakeMeleeDamage(2)
	s.w.Regenerate()
	s.Assert().Equal(100, s.p.CurrentHealth())
}

// Nobody heals while fighting: including the mob on the other side.
func (s *regenSuite) TestNotWhileFighting() {
	target, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)
	s.Require().NoError(s.w.fightLedger.Fight(s.p, target))
	s.p.TakeMeleeDamage(50)
	target.TakeMeleeDamage(10)

	s.w.Regenerate()

	s.Assert().Equal(50, s.p.CurrentHealth())
	s.Assert().Equal(15, target.CurHealth)
}

func (s *regenSuite) TestMobsHealToo() {
	target, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)
	target.TakeMeleeDamage(10) // 25 max, 15 left

	s.w.Regenerate()

	s.Assert().Equal(16, target.CurHealth, "5% of 25 is 1")
}

// Mana comes back in a fight -- the healer is busy the whole fight -- while
// health still doesn't.
func (s *regenSuite) TestManaEvenWhileFighting() {
	target, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)
	s.Require().NoError(s.w.fightLedger.Fight(s.p, target))
	s.p.TakeMeleeDamage(50)
	s.Require().True(s.p.SpendMana(60))

	s.w.Regenerate()

	s.Assert().Equal(45, s.p.CurrentMana(), "5% of 100")
	s.Assert().Equal(50, s.p.CurrentHealth(), "still no health mid-fight")
}
