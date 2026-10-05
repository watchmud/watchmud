package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

// The reported pack: a waterskin, two leather caps and leather boots --
// the caps and the boots all answer to "leather", the caps listed first.
type wearSuite struct {
	worldTestSuite
	cap1, cap2, boots *object.Instance
}

func TestWearSuite(t *testing.T) {
	suite.Run(t, new(wearSuite))
}

func (s *wearSuite) item(name, short string, aliases []string, slot rules.EquipmentSlot) *object.Instance {
	d := object.NewDefinition(name, name, "wrathrock", rules.ObjectCategoryArmor, aliases,
		short, short+" is here.", slot, rules.ArmorTypeLeather, nil)
	inst := object.NewInstance(uuid.New(), d)
	s.Require().NoError(s.p.Inventory().Add(inst))
	return inst
}

func (s *wearSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.item("waterskin", "a waterskin", nil, rules.SlotNone)
	s.cap1 = s.item("leather cap", "a leather cap", []string{"cap", "leather"}, rules.EquipmentSlot("head"))
	s.cap2 = s.item("leather cap", "a leather cap", []string{"cap", "leather"}, rules.EquipmentSlot("head"))
	s.boots = s.item("leather boots", "a pair of leather boots", []string{"boots", "leather"}, rules.EquipmentSlot("feet"))
}

func (s *wearSuite) wear(target string) {
	s.T().Helper()
	s.r.Clear()
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Wear{Target: target})))
}

func (s *wearSuite) on(slot string) *object.Instance {
	return s.p.Equipment().At(rules.EquipmentSlot(slot))
}

// what went on is named, not just "Done."
func (s *wearSuite) TestNamesWhatItWore() {
	s.wear("boots")

	s.Assert().Same(s.boots, s.on("feet"))
	s.Assert().Equal(event.Worn{Item: "a pair of leather boots"}, sent[event.Worn](s.T(), s.r, 0))
}

// "leather" with the head already covered goes on to the next leather thing
// that can go on -- the boots -- rather than refusing over a second cap
func (s *wearSuite) TestPassesOverWhatCantGoOn() {
	s.wear("leather")
	s.Assert().Same(s.cap1, s.on("head"))

	s.wear("leather")

	s.Assert().Same(s.boots, s.on("feet"))
	s.Assert().Same(s.cap1, s.on("head"), "the first cap stays on")
}

// 2.leather is the second leather thing in the pack, as every other command counts
func (s *wearSuite) TestNumbered() {
	s.wear("3.leather")

	s.Assert().Same(s.boots, s.on("feet"))
}

// everything that answers is on or blocked: say which, truthfully
func (s *wearSuite) TestSlotTaken() {
	s.wear("cap")

	s.wear("cap")

	s.Assert().Equal(event.LocationInUse, sent[event.Failed](s.T(), s.r, 0).Code, "the second cap: the head is taken")
}

func (s *wearSuite) TestAlreadyWearingIt() {
	s.wear("boots")

	s.wear("boots")

	s.Assert().Equal(event.InUse, sent[event.Failed](s.T(), s.r, 0).Code, "the only boots are on")
}

// asked by number for one that's on, that's the answer
func (s *wearSuite) TestNumberedAlreadyOn() {
	s.wear("cap")

	s.wear("1.cap")

	s.Assert().Equal(event.InUse, sent[event.Failed](s.T(), s.r, 0).Code)
}

func (s *wearSuite) TestNotFound() {
	s.wear("helmet")
	s.Assert().Equal(event.TargetNotFound, sent[event.Failed](s.T(), s.r, 0).Code)

	s.wear("4.leather")
	s.Assert().Equal(event.TargetNotFound, sent[event.Failed](s.T(), s.r, 0).Code)
}

func (s *wearSuite) TestCantWear() {
	s.wear("waterskin")
	s.Assert().Equal(event.CantWearThat, sent[event.Failed](s.T(), s.r, 0).Code)
}

func (s *wearSuite) TestNoTarget() {
	s.wear("")
	s.Assert().Equal(event.NoTarget, sent[event.Failed](s.T(), s.r, 0).Code)
}
