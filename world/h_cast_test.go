package world

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

type handleCastSuite struct {
	worldTestSuite
	clock time.Time
	bob   *player.Player
	bobR  *player.Recorder
}

func TestHandleCastSuite(t *testing.T) {
	suite.Run(t, new(handleCastSuite))
}

func (s *handleCastSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.clock = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	s.w.now = func() time.Time { return s.clock }
	s.bobR = &player.Recorder{}
	s.bob = player.NewTestPlayer(uuid.New(), "bob", s.bobR)
	s.w.PlacePlayer(s.bob, s.w.StartRoom)
}

// hold a censer granting heal, at this power
func (s *handleCastSuite) holdCenser(power int) *object.Instance {
	s.T().Helper()
	d := object.NewTestDefinition(s.T(), "censer", rules.SlotHold, rules.ObjectCategoryOther, rules.ArmorTypeNone)
	d.Abilities = []string{"heal"}
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = power
	s.Require().NoError(s.p.Inventory().Add(inst))
	s.p.Equipment().Equip(rules.SlotHold, inst)
	return inst
}

func (s *handleCastSuite) cast(ability, target string) {
	s.T().Helper()
	s.r.Sent, s.bobR.Sent = nil, nil
	cmd := command.Cast{Ability: ability, Target: target}
	s.w.handleCast(s.handlerParameter(cmd), cmd)
}

func (s *handleCastSuite) failed() event.ResultCode {
	s.T().Helper()
	return sent[event.Failed](s.T(), s.r, 0).Code
}

func (s *handleCastSuite) TestHealSelf() {
	s.holdCenser(1)
	s.p.TakeMeleeDamage(50)

	s.cast("heal", "")

	s.Assert().Equal(62, s.p.CurrentHealth(), "10 + 2 per power")
	s.Assert().Equal(80, s.p.CurrentMana())
	s.Assert().Equal(event.Healed{Actor: "testdood", Target: "testdood", Amount: 12},
		sent[event.Healed](s.T(), s.r, 0))
}

func (s *handleCastSuite) TestHealOther() {
	s.holdCenser(3)
	s.bob.TakeMeleeDamage(50)

	s.cast("heal", "bob")

	s.Assert().Equal(66, s.bob.CurrentHealth())
	want := event.Healed{Actor: "testdood", Target: "bob", Amount: 16}
	s.Assert().Equal(want, sent[event.Healed](s.T(), s.r, 0))
	s.Assert().Equal(want, sent[event.Healed](s.T(), s.bobR, 0), "the room sees it")
}

// Review Focus 1 and 2: case, and yourself by name
func (s *handleCastSuite) TestCaseAndOwnName() {
	s.holdCenser(1)
	s.bob.TakeMeleeDamage(50)
	s.cast("HEAL", "Bob")
	s.Assert().Equal(62, s.bob.CurrentHealth())

	s.clock = s.clock.Add(time.Minute)
	s.p.TakeMeleeDamage(50)
	s.cast("heal", "testdood")
	s.Assert().Equal(62, s.p.CurrentHealth())
}

// a wasted heal is still a heal: it spends, and says it was wasted
func (s *handleCastSuite) TestUnhurtStillSpends() {
	s.holdCenser(1)

	s.cast("heal", "bob")

	s.Assert().Equal(80, s.p.CurrentMana())
	s.Assert().Zero(sent[event.Healed](s.T(), s.r, 0).Amount)
	s.cast("heal", "bob")
	s.Assert().Equal(event.NotReady, s.failed(), "and the cooldown started")
}

func (s *handleCastSuite) TestMidFight() {
	s.holdCenser(1)
	target, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)
	s.Require().NoError(s.w.fightLedger.Fight(s.p, target))
	s.p.TakeMeleeDamage(50)

	s.cast("heal", "")

	s.Assert().Equal(62, s.p.CurrentHealth())
}

func (s *handleCastSuite) TestCooldown() {
	s.holdCenser(1)
	s.cast("heal", "")

	s.clock = s.clock.Add(9 * time.Second)
	s.cast("heal", "")
	s.Assert().Equal(event.NotReady, s.failed())
	s.Assert().Equal(80, s.p.CurrentMana(), "refused casts cost nothing")

	s.clock = s.clock.Add(time.Second)
	s.cast("heal", "")
	s.Assert().Equal(60, s.p.CurrentMana(), "ready again at exactly the cooldown")
}

func (s *handleCastSuite) TestRefusals() {
	s.cast("", "")
	s.Assert().Equal(event.NoTarget, s.failed())

	s.cast("fireball", "")
	s.Assert().Equal(event.UnknownAbility, s.failed())

	s.cast("heal", "")
	s.Assert().Equal(event.NotGranted, s.failed(), "nothing equipped")

	s.holdCenser(1)
	s.cast("heal", "nobody")
	s.Assert().Equal(event.TargetNotFound, s.failed())

	s.Require().True(s.p.SpendMana(85))
	s.cast("heal", "")
	s.Assert().Equal(event.NotEnoughMana, s.failed())
	s.Assert().Equal(15, s.p.CurrentMana())
}

// a censer in your pack does nothing
func (s *handleCastSuite) TestCarriedIsNotEquipped() {
	censer := s.holdCenser(1)
	s.p.Equipment().Unequip(rules.SlotHold)
	_, carried := s.p.Inventory().Get(censer.Id)
	s.Require().True(carried)

	s.cast("heal", "")
	s.Assert().Equal(event.NotGranted, s.failed())
}

// Review Focus 3: no heals through walls, and a miss costs nothing
func (s *handleCastSuite) TestOtherRoom() {
	s.holdCenser(1)
	smithy, found := s.w.findRoomById("wrathrock", "smithy")
	s.Require().True(found)
	s.w.movePlayerMagically(s.bob, smithy)

	s.cast("heal", "bob")

	s.Assert().Equal(event.TargetNotFound, s.failed())
	s.Assert().Equal(100, s.p.CurrentMana())
	s.Assert().True(s.p.ReadyAt("heal").IsZero())
}

// every ability in the catalog has an effect, or the world won't build
func (s *handleCastSuite) TestEveryAbilityHasAnEffect() {
	s.Assert().NoError(checkEffects(s.w.content.Catalog))
	cat, err := rules.NewTestCatalog()
	s.Require().NoError(err)
	s.Require().NoError(cat.SetAbilities([]*rules.Ability{{Id: "levitate", Name: "levitate", Target: rules.TargetSelf}}))
	s.Assert().ErrorContains(checkEffects(cat), "levitate")
}
