package world

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

// fireball in testcontent: 40 mana, 60s cooldown, 4 + 2 per power to every
// foe in the room -- 6 at power 1. The start room has a Target Drone and a
// Little Drone, 25 health each, placed in that order.
type fireballSuite struct {
	worldTestSuite
	clock  time.Time
	target *mobile.Instance
	little *mobile.Instance
}

func TestFireballSuite(t *testing.T) {
	suite.Run(t, new(fireballSuite))
}

func (s *fireballSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.clock = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	s.w.now = func() time.Time { return s.clock }
	var found bool
	s.target, found = s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	s.little, found = s.w.StartRoom.FindMobile("little")
	s.Require().True(found)
}

// wield a staff granting fireball, at power 1, able to take 10 points of wear
func (s *fireballSuite) wieldStaff() *object.Instance {
	s.T().Helper()
	d := object.NewTestDefinition(s.T(), "staff", rules.SlotWield, rules.ObjectCategoryWeapon, rules.ArmorTypeNone)
	d.Damage = "1d6"
	d.Abilities = []string{"fireball"}
	d.MaxDurability = 10
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = 1
	s.Require().NoError(s.p.Inventory().Add(inst))
	s.p.Equipment().Equip(rules.SlotWield, inst)
	return inst
}

func (s *fireballSuite) fireball() {
	s.T().Helper()
	s.r.Clear()
	cmd := command.Cast{Ability: "fireball"}
	s.w.handleCast(s.handlerParameter(cmd), cmd)
}

func (s *fireballSuite) failed() event.ResultCode {
	s.T().Helper()
	return sent[event.Failed](s.T(), s.r, 0).Code
}

func (s *fireballSuite) assertNothingSpent() {
	s.T().Helper()
	s.Assert().Equal(100, s.p.CurrentMana(), "a refusal costs no mana")
	s.Assert().True(s.p.ReadyAt("fireball").IsZero(), "and starts no cooldown")
}

// how many of T the player was sent
func count[T any](r *player.Recorder) int {
	n := 0
	for _, m := range r.Sent {
		if _, ok := m.(T); ok {
			n++
		}
	}
	return n
}

func (s *fireballSuite) TestFireballHitsEveryMob() {
	s.wieldStaff()

	s.fireball()

	s.Assert().Equal(event.Fireballed{Actor: "testdood", Targets: []string{"Target Drone", "Little Drone"}, Damage: 6},
		sent[event.Fireballed](s.T(), s.r, 0))
	s.Assert().Equal(19, s.target.CurHealth)
	s.Assert().Equal(19, s.little.CurHealth)
	s.Assert().Equal(60, s.p.CurrentMana())
	s.Assert().Equal(s.clock.Add(60*time.Second), s.p.ReadyAt("fireball"))
}

// it's dangerous: everything it hits comes for you
func (s *fireballSuite) TestFireballDrawsThemAllIn() {
	s.wieldStaff()

	s.fireball()

	s.Require().True(s.w.fightLedger.IsFighting(s.p))
	s.Assert().Equal(s.target, s.w.fightLedger.GetFight(s.p).Fightee, "you swing at the first")
	s.Require().True(s.w.fightLedger.IsFighting(s.target))
	s.Assert().Equal(s.p, s.w.fightLedger.GetFight(s.target).Fightee)
	s.Require().True(s.w.fightLedger.IsFighting(s.little))
	s.Assert().Equal(s.p, s.w.fightLedger.GetFight(s.little).Fightee)
}

// a mob already fighting someone else is hit, and stays on them
func (s *fireballSuite) TestFireballLeavesAnotherFightAlone() {
	s.wieldStaff()
	bob := player.NewTestPlayer(uuid.New(), "bob", &player.Recorder{})
	s.w.PlacePlayer(bob, s.w.StartRoom)
	s.Require().NoError(s.w.startFight(bob, s.little))

	s.fireball()

	s.Assert().Equal(19, s.little.CurHealth)
	s.Assert().Equal(bob, s.w.fightLedger.GetFight(s.little).Fightee, "still on bob")
	s.Assert().Equal(s.p, s.w.fightLedger.GetFight(s.target).Fightee, "the idle one turns on you")
}

func (s *fireballSuite) TestFireballNeverTouchesPlayers() {
	s.wieldStaff()
	bob := player.NewTestPlayer(uuid.New(), "bob", &player.Recorder{})
	s.w.PlacePlayer(bob, s.w.StartRoom)

	s.fireball()

	s.Assert().Equal(100, bob.CurrentHealth())
	s.Assert().False(s.w.fightLedger.InFight(bob))
	s.Assert().Equal(100, s.p.CurrentHealth())
}

func (s *fireballSuite) TestFireballSkipsCantFightMobs() {
	s.wieldStaff()
	s.little.Definition.SetFlag(rules.MobileFlagPlayerCantFight)

	s.fireball()

	s.Assert().Equal([]string{"Target Drone"}, sent[event.Fireballed](s.T(), s.r, 0).Targets)
	s.Assert().Equal(25, s.little.CurHealth)
	s.Assert().False(s.w.fightLedger.InFight(s.little))
}

func (s *fireballSuite) TestFireballNoFoes() {
	s.wieldStaff()
	s.target.Definition.SetFlag(rules.MobileFlagPlayerCantFight)
	s.little.Definition.SetFlag(rules.MobileFlagPlayerCantFight)

	s.fireball()

	s.Assert().Equal(event.NoFoes, s.failed())
	s.assertNothingSpent()
}

func (s *fireballSuite) TestFireballNoFightRoom() {
	s.wieldStaff()
	s.w.StartRoom.SetFlag(rules.RoomFlagNoFight)

	s.fireball()

	s.Assert().Equal(event.NoFightRoom, s.failed())
	s.Assert().Equal(25, s.target.CurHealth)
	s.assertNothingSpent()
}

// the fire, then the deaths in the order they burned, and no fight left
func (s *fireballSuite) TestFireballKills() {
	s.wieldStaff()
	s.target.CurHealth = 5
	s.little.CurHealth = 5

	s.fireball()

	s.Assert().IsType(event.Fireballed{}, s.r.Sent[0])
	s.Assert().Equal(event.Died{Target: "Target Drone"}, sent[event.Died](s.T(), s.r, 1))
	s.Assert().Equal(event.Died{Target: "Little Drone"}, sent[event.Died](s.T(), s.r, 2))
	s.Assert().Empty(s.w.StartRoom.Mobiles())
	s.Assert().False(s.w.fightLedger.InFight(s.p), "nobody left to fight")
}

// a summoner burned to death crumbles its summons mid-cast; a summon that
// burned too is gone already, and mustn't die a second time
func (s *fireballSuite) TestFireballKillsSummonerAndSummons() {
	s.wieldStaff()
	s.Require().Equal(1, s.w.summon(s.target, s.little.Definition, 1))
	for _, m := range s.w.StartRoom.Mobiles() {
		m.CurHealth = 5
	}

	s.fireball()

	s.Assert().Equal(3, len(sent[event.Fireballed](s.T(), s.r, 0).Targets), "the summon burns too")
	s.Assert().Equal(2, count[event.Died](s.r), "the drone and the little one die")
	s.Assert().Equal(1, count[event.Crumbled](s.r), "the summon crumbles, once")
	s.Assert().Empty(s.w.StartRoom.Mobiles())
	s.Assert().False(s.w.fightLedger.InFight(s.p))
}

// one cast, one point off the staff, however many it hits
func (s *fireballSuite) TestFireballWearsOnce() {
	staff := s.wieldStaff()

	s.fireball()

	s.Assert().Equal(9, staff.Durability)
}
