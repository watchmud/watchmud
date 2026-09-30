package player

import (
	"slices"
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

type PlayerSuite struct {
	suite.Suite
	p *Player
	r *Recorder
}

func TestPlayerSuite(t *testing.T) {
	suite.Run(t, new(PlayerSuite))
}

func (s *PlayerSuite) SetupTest() {
	s.r = &Recorder{}
	s.p = NewTestPlayer(uuid.New(), "testdood", s.r)
}

func (s *PlayerSuite) TestAddInventory_New() {
	// old test, do not trust it
	knife := object.MakeTestKnife(s.T())

	s.Assert().NoError(s.p.inventory.Add(knife))

	inventory := slices.Collect(s.p.inventory.All())
	s.Assert().Equal(1, len(inventory))
	obj := inventory[0]
	s.Assert().Equal(inventory[0], obj)
}

func (s *PlayerSuite) TestMeleeDamage() {
	// old test, do not trust it

	startingHealth := s.p.curHealth

	isDead := s.p.TakeMeleeDamage(5)

	s.Assert().False(isDead)
	s.Assert().Equal(startingHealth-5, s.p.curHealth)
}

func (s *PlayerSuite) TestFatalMeleeDamage() {
	// old test, do not trust it

	startingHealth := s.p.curHealth

	isDead := s.p.TakeMeleeDamage(startingHealth)

	s.Assert().True(isDead)
	s.Assert().Equal(0, s.p.curHealth)
}

func (s *PlayerSuite) TestOverwhelminglyFatalMeleeDamage() {
	// old test, do not trust it

	startingHealth := s.p.curHealth

	isDead := s.p.TakeMeleeDamage(startingHealth * 2)

	s.Assert().True(isDead)
	s.Assert().True(s.p.curHealth < 0)
}

func (s *PlayerSuite) TestIsDead() {
	// old test, do not trust it

	s.Assert().False(s.p.Dead())
	s.p.curHealth = 0
	s.Assert().True(s.p.Dead())
}

// Being a wizard is on the record, so it has to come back off it -- a save
// that forgot it would demote every wizard at the next timed save.
func (s *PlayerSuite) TestWizardSurvivesTheRecord() {
	p := NewTestPlayer(uuid.New(), "gandalf", &Recorder{})
	s.Assert().False(p.IsWizard(), "nobody starts as one")
	p.SetWizard(true)

	rec := p.Record()
	s.Assert().True(rec.Wizard)

	cat, err := rules.NewTestCatalog()
	s.Require().NoError(err)
	back, err := FromRecord(rec, &Recorder{}, cat, nil)
	s.Require().NoError(err)
	s.Assert().True(back.IsWizard())
}

// The bot flag is on the record like Wizard, for the same reason: a save
// that forgot it would unlabel every bot at the next timed save.
func (s *PlayerSuite) TestBotSurvivesTheRecord() {
	p := NewTestPlayer(uuid.New(), "wren", &Recorder{})
	s.Assert().False(p.IsBot(), "nobody starts as one")
	p.SetBot(true)

	rec := p.Record()
	s.Assert().True(rec.Bot)

	cat, err := rules.NewTestCatalog()
	s.Require().NoError(err)
	back, err := FromRecord(rec, &Recorder{}, cat, nil)
	s.Require().NoError(err)
	s.Assert().True(back.IsBot())
}
