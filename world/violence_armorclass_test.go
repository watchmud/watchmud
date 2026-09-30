package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/combat"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/testdice"
)

// Armor class from the equipment a player is actually wearing, all the way
// through to whether a swing lands in a real fight. Everything below the top
// of this file is covered elsewhere -- the armor table in object/, the
// comparison in combat/ -- and none of that proves the number reaches the
// fight, which is the part that was wrong for mobs.
//
// Not the shared worldTestSuite: player.NewTestPlayer builds its own catalog
// with an empty armor table, so armor worn by that player is worth nothing.
// These need the world's real catalog.
type violenceArmorClassSuite struct {
	suite.Suite
	w     *World
	p     *player.Player
	r     *player.Recorder
	dice  *testdice.LoadedDice
	drone combat.Combatant
}

func TestViolenceArmorClassSuite(t *testing.T) {
	suite.Run(t, new(violenceArmorClassSuite))
}

func (s *violenceArmorClassSuite) SetupTest() {
	w, err := NewTestWorld()
	s.Require().NoError(err)
	s.w = w

	cat := w.content.Catalog
	s.r = &player.Recorder{}
	s.p = player.New(uuid.New(), "victim", "", s.r, cat.DefaultLineage(), cat)
	s.w.PlacePlayer(s.p, s.w.StartRoom)

	drone, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)
	// Level with the unequipped player, so these measure armor class and
	// nothing else; the power difference has its own tests. NewTestWorld
	// loads content fresh each time, so this doesn't leak between tests.
	drone.Definition.Power = 0
	s.drone = drone

	s.dice = testdice.New()
	s.w.roller = s.dice
}

// wear a real piece of armor, valued by the real table
func (s *violenceArmorClassSuite) wear(slot rules.EquipmentSlot, name string, t rules.ArmorType) {
	d := object.NewDefinition(name, name, "wrathrock", rules.ObjectCategoryArmor,
		nil, name, name+" is here.", slot, t, rules.EmptyObjectBehaviors)
	s.p.Equipment().Equip(slot, object.NewInstance(uuid.New(), d))
}

// oneWayFight leaves exactly one swing to take.
//
// FightLedger.Fight writes both directions, and DoViolence walks them in map
// order, so a two-sided fight is two swings in an unpredictable order --
// which loaded dice cannot be aimed at. Ending the other side leaves one.
func (s *violenceArmorClassSuite) oneWayFight(attacker, defender combat.Combatant) {
	s.T().Helper()
	s.w.fightLedger = combat.NewFightLedger()
	s.Require().NoError(s.w.fightLedger.Fight(attacker, defender))
	s.w.fightLedger.EndFight(defender)
}

// swing: the drone attacks the player with this d20, and this damage roll if
// it lands. Returns whether the room saw a hit.
func (s *violenceArmorClassSuite) swing(d20 int) bool {
	s.dice.Load([]int{d20, 3})
	s.oneWayFight(s.drone, s.p)
	s.r.Sent = nil

	// pulse 5: CanDoViolence wants three seconds since PulseCountNever
	s.w.DoViolence(5)

	for _, msg := range s.r.Sent {
		if struck, ok := msg.(event.Struck); ok && struck.Target == s.p.Name() {
			return struck.Hit
		}
	}
	s.Require().Fail("nobody swung at the player")
	return false
}

func (s *violenceArmorClassSuite) TestUnarmoredIsTheBaseline() {
	s.Assert().Equal(rules.BaseArmorClass, s.p.ArmorClass())
}

func (s *violenceArmorClassSuite) TestWornArmorRaisesArmorClass() {
	s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate)  // body plate: 4
	s.wear(rules.SlotHead, "iron helmet", rules.ArmorTypePlate) // head plate: 1

	s.Assert().Equal(rules.BaseArmorClass+5, s.p.ArmorClass())
}

// The boundary, in a real fight: the roll that equals armor class lands, the
// one below it doesn't.
func (s *violenceArmorClassSuite) TestTheRollMustMeetArmorClass() {
	s.Assert().True(s.swing(rules.BaseArmorClass), "a 10 lands on an unarmored player")
	s.Assert().False(s.swing(rules.BaseArmorClass-1), "a 9 does not")
}

// The point of wearing any of it: the identical roll that hit you naked
// misses once you're in plate.
func (s *violenceArmorClassSuite) TestArmorTurnsAHitIntoAMiss() {
	s.Require().True(s.swing(12), "12 lands on an unarmored player")

	s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate)
	s.Require().Equal(rules.BaseArmorClass+4, s.p.ArmorClass())

	s.Assert().False(s.swing(12), "and misses the same player in plate")
	s.Assert().True(s.swing(14), "which a 14 still beats")
}

// Both directions of the same fight are measured the same way: the mob's
// armor class is absolute too, so the drone at "ac": 10 is hit by a 10 and
// missed by a 9, exactly like the unarmored player.
func (s *violenceArmorClassSuite) TestMobArmorClassIsOnTheSameScale() {
	s.Assert().Equal(rules.BaseArmorClass, s.drone.ArmorClass())

	attack := func(d20 int) bool {
		s.dice.Load([]int{d20, 3})
		s.oneWayFight(s.p, s.drone)
		s.r.Sent = nil
		s.w.DoViolence(5)
		for _, msg := range s.r.Sent {
			if struck, ok := msg.(event.Struck); ok && struck.Target == s.drone.Name() {
				return struck.Hit
			}
		}
		s.Require().Fail("nobody swung at the drone")
		return false
	}

	s.Assert().True(attack(10))
	s.Assert().False(attack(9))
}

// Power reaches a real fight. The drone ten above the player gets +5, so a 5
// lands on an unarmored player that only a 10 could hit on level terms.
func (s *violenceArmorClassSuite) TestPowerReachesTheFight() {
	s.drone.(*mobile.Instance).Definition.Power = 10

	s.Assert().True(s.swing(5), "5 + 5 meets AC 10")
	s.Assert().False(s.swing(4), "4 + 5 does not")
}
