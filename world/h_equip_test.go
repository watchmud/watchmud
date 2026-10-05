package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

type HandleEquipSuite struct {
	suite.Suite
	w *World
	r *player.Recorder
	p *player.Player
	c *gameserver.TestConn
}

func TestHandleEquipSuite(t *testing.T) {
	suite.Run(t, new(HandleEquipSuite))
}

func (s *HandleEquipSuite) SetupTest() {
	s.w, _ = NewTestWorld()
	s.r = &player.Recorder{}
	s.p = player.NewTestPlayer(uuid.New(), "foo", s.r)
	s.w.PlacePlayer(s.p, s.w.StartRoom)
	s.c = gameserver.NewTestConn(s.p)
}

func (s *HandleEquipSuite) equip(cmd command.Equip) {
	s.T().Helper()
	s.w.handleEquip(gameserver.NewHandlerParameter(s.c, cmd), cmd)
}

func (s *HandleEquipSuite) TestNoSlot() {
	s.equip(command.Equip{})

	s.Assert().Equal(1, len(s.r.Sent))
	failed := s.r.Sent[0].(event.Failed)
	s.Assert().Equal("equip", failed.Verb)
	s.Assert().Equal(event.NoSlotGiven, failed.Code)
}

func (s *HandleEquipSuite) TestNoTarget() {
	s.equip(command.Equip{Slot: rules.SlotWield})

	s.Assert().Equal(1, len(s.r.Sent))
	failed := s.r.Sent[0].(event.Failed)
	s.Assert().Equal(event.NoTarget, failed.Code)
}

// wield gets the same rules as wear: two knives and a dagger, all answering
// to "blade", and a cap that answers to it too -- listed first.
func (s *HandleEquipSuite) pack() (cap, knife1, knife2, dagger *object.Instance) {
	s.T().Helper()
	add := func(name, short string, aliases []string, slot rules.EquipmentSlot) *object.Instance {
		d := object.NewDefinition(name, name, "wrathrock", rules.ObjectCategoryWeapon, aliases,
			short, short+" is here.", slot, rules.ArmorTypeNone, nil)
		inst := object.NewInstance(uuid.New(), d)
		s.Require().NoError(s.p.Inventory().Add(inst))
		return inst
	}
	cap = add("bladed cap", "a bladed cap", []string{"cap", "blade"}, rules.EquipmentSlot("head"))
	knife1 = add("knife", "a knife", []string{"blade"}, rules.SlotWield)
	knife2 = add("knife", "a knife", []string{"blade"}, rules.SlotWield)
	dagger = add("dagger", "a dagger", []string{"blade"}, rules.SlotWield)
	return
}

func (s *HandleEquipSuite) wield(target string) {
	s.T().Helper()
	s.r.Clear()
	s.equip(command.Equip{Target: target, Slot: rules.SlotWield})
}

func (s *HandleEquipSuite) failed() event.ResultCode {
	s.T().Helper()
	return sent[event.Failed](s.T(), s.r, 0).Code
}

// "blade" passes over the cap, which can't be wielded, to the first knife --
// and names it
func (s *HandleEquipSuite) TestWieldPassesOverWhatCantBeWielded() {
	_, knife1, _, _ := s.pack()

	s.wield("blade")

	s.Assert().Same(knife1, s.p.Equipment().At(rules.SlotWield))
	s.Assert().Equal(event.Equipped{Item: "a knife"}, sent[event.Equipped](s.T(), s.r, 0))
}

// numbered, counted through the pack: 4.blade is the dagger
func (s *HandleEquipSuite) TestWieldNumbered() {
	_, _, _, dagger := s.pack()

	s.wield("4.blade")

	s.Assert().Same(dagger, s.p.Equipment().At(rules.SlotWield))
}

// asked by number for the cap: it doesn't go in the hand
func (s *HandleEquipSuite) TestWieldNumberedWrongSlot() {
	s.pack()

	s.wield("1.blade")

	s.Assert().Equal(event.CantWearThere, s.failed())
}

// a knife in hand: another blade is blocked by it, said as such
func (s *HandleEquipSuite) TestWieldSlotTaken() {
	s.pack()
	s.wield("knife")

	s.wield("blade")

	s.Assert().Equal(event.LocationInUse, s.failed())
}

// the only dagger, already in hand
func (s *HandleEquipSuite) TestWieldAlreadyWielding() {
	s.pack()
	s.wield("dagger")

	s.wield("dagger")

	s.Assert().Equal(event.InUse, s.failed())
}

func (s *HandleEquipSuite) TestWieldNotFound() {
	s.pack()

	s.wield("5.blade")
	s.Assert().Equal(event.TargetNotFound, s.failed())

	s.wield("sword")
	s.Assert().Equal(event.TargetNotFound, s.failed())
}
