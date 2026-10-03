package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

type noHassleSuite struct {
	worldTestSuite
	drone *mobile.Instance
}

func TestNoHassleSuite(t *testing.T) {
	suite.Run(t, new(noHassleSuite))
}

// the target drone in the start room, made aggressive
func (s *noHassleSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	drone, found := s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	drone.Definition.SetFlag(rules.MobileFlagAggressive)
	s.drone = drone
}

func (s *noHassleSuite) protect() {
	s.p.SetWizard(true)
	s.p.SetNoHassle(true)
}

func (s *noHassleSuite) TestWizardIsLeftAlone() {
	s.protect()

	s.w.doMobAggro(s.drone)

	s.Assert().False(s.w.fightLedger.InFight(s.p))
	s.Assert().False(s.w.fightLedger.IsFighting(s.drone), "and nobody else is here to pick on")
}

// the wizard standing first in the room doesn't shield whoever is next
func (s *noHassleSuite) TestMortalBesideAWizardIsAttacked() {
	s.protect()
	mortal := player.NewTestPlayer(uuid.New(), "mortal", nil)
	s.w.PlacePlayer(mortal, s.w.StartRoom)

	s.w.doMobAggro(s.drone)

	s.Require().True(s.w.fightLedger.IsFighting(s.drone))
	s.Assert().Equal(mortal, s.w.fightLedger.GetFight(s.drone).Fightee)
	s.Assert().False(s.w.fightLedger.InFight(s.p))
}

// switched off, so a wizard can test aggro on their own character
func (s *noHassleSuite) TestWizardWithItOffIsAttacked() {
	s.p.SetWizard(true)

	s.w.doMobAggro(s.drone)

	s.Assert().True(s.w.fightLedger.InFight(s.p))
}

// the flag means nothing without the wizard bit
func (s *noHassleSuite) TestFlagAloneProtectsNobody() {
	s.p.SetNoHassle(true)

	s.w.doMobAggro(s.drone)

	s.Assert().True(s.w.fightLedger.InFight(s.p))
}

// every session starts protected for a wizard, and never for anyone else
func (s *noHassleSuite) TestArriveSwitchesItOnForWizards() {
	s.p.SetWizard(true)
	s.w.Arrive(s.p)
	s.Assert().True(s.p.NoHassle())

	mortal := player.NewTestPlayer(uuid.New(), "mortal", nil)
	mortal.SetNoHassle(true)
	s.w.PlacePlayer(mortal, s.w.StartRoom)
	s.w.Arrive(mortal)
	s.Assert().False(mortal.NoHassle())
}

func (s *noHassleSuite) noHassle(setting string) {
	s.T().Helper()
	s.r.Clear()
	cmd := command.NoHassle{Setting: setting}
	s.w.handleNoHassle(s.handlerParameter(cmd), cmd)
}

func (s *noHassleSuite) TestCommand() {
	s.p.SetWizard(true)

	s.noHassle("")
	s.Assert().True(s.p.NoHassle(), "bare flips it")
	s.Assert().Equal(event.NoHassle{On: true}, sent[event.NoHassle](s.T(), s.r, 0))

	s.noHassle("off")
	s.Assert().False(s.p.NoHassle())
	s.noHassle("off")
	s.Assert().False(s.p.NoHassle(), "off is off, not a flip")
	s.noHassle("on")
	s.Assert().True(s.p.NoHassle())

	s.noHassle("sideways")
	s.Assert().Equal(event.BadRequest, sent[event.Failed](s.T(), s.r, 0).Code)
	s.Assert().True(s.p.NoHassle(), "unchanged")
}
