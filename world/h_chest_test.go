package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/lock"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

// A strongbox on Temple Square's floor, locked, with a knife in it.
type chestSuite struct {
	worldTestSuite
	box *object.Instance
}

func TestChestSuite(t *testing.T) {
	suite.Run(t, new(chestSuite))
}

func (s *chestSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	d := object.NewDefinition("strongbox", "strongbox", "wrathrock", rules.ObjectCategoryOther,
		[]string{"box"}, "a strongbox", "A strongbox sits here.", rules.SlotNone, rules.ArmorTypeNone,
		[]rules.ObjectBehavior{rules.ObjectBehaviorNoTake})
	d.Container = &object.ContainerSpec{Initial: lock.State{Closed: true, Locked: true}, Key: "wrathrock/box_key"}
	s.box = object.NewInstance(uuid.New(), d)
	knife := object.NewDefinition("knife", "knife", "wrathrock", rules.ObjectCategoryWeapon, nil,
		"a knife", "A knife is here.", rules.SlotWield, rules.ArmorTypeNone, nil)
	s.Require().NoError(s.box.Contents.Add(object.NewInstance(uuid.New(), knife)))
	s.Require().NoError(s.w.StartRoom.Inventory.Add(s.box))
}

func (s *chestSuite) carryKey() {
	d := object.NewDefinition("box_key", "box key", "wrathrock", rules.ObjectCategoryOther,
		[]string{"key"}, "a small key", "A small key is here.", rules.SlotNone, rules.ArmorTypeNone, nil)
	s.Require().NoError(s.p.Inventory().Add(object.NewInstance(uuid.New(), d)))
}

func (s *chestSuite) do(cmd command.Command) {
	s.T().Helper()
	s.r.Clear()
	s.w.HandleIncomingMessage(s.handlerParameter(cmd))
}

func (s *chestSuite) failed() event.ResultCode {
	s.T().Helper()
	return sent[event.Failed](s.T(), s.r, 0).Code
}

func (s *chestSuite) TestClosedHidesItsContents() {
	s.do(command.Look{Target: "box", In: true})
	s.Assert().Equal(event.ContainerClosed, s.failed())
	s.do(command.Get{Target: "knife", From: "box"})
	s.Assert().Equal(event.ContainerClosed, s.failed())
}

func (s *chestSuite) TestLockedNeedsTheKey() {
	s.do(command.Open{Target: "box"})
	s.Assert().Equal(event.Locked, s.failed())
	s.do(command.Unlock{Target: "box"})
	s.Assert().Equal(event.NoKey, s.failed())
}

func (s *chestSuite) TestUnlockOpenAndTake() {
	s.carryKey()

	s.do(command.Unlock{Target: "strongbox"})
	s.Assert().Equal(event.ContainerChanged{Actor: "testdood", Container: "strongbox", Change: event.DoorUnlocked},
		sent[event.ContainerChanged](s.T(), s.r, 0))
	s.do(command.Open{Target: "box"})
	s.Assert().False(s.box.Closed())

	s.do(command.Get{Target: "knife", From: "box"})
	sent[event.Got](s.T(), s.r, 0)

	s.do(command.Close{Target: "box"})
	s.do(command.Lock{Target: "box"})
	s.Assert().True(s.box.Lock.Locked)
}

// a chest is furniture
func (s *chestSuite) TestStaysPut() {
	s.do(command.Get{Target: "box"})
	_, onFloor := s.w.StartRoom.Inventory.Get(s.box.Id)
	s.Assert().True(onFloor)
}

// a corpse has no lid: open has nothing to say to it
func (s *chestSuite) TestACorpseHasNoLid() {
	s.do(command.Open{Target: "knife"})
	s.Assert().Equal(event.NoDoor, s.failed())
}

func (s *chestSuite) open() {
	s.T().Helper()
	s.box.Lock.Locked, s.box.Lock.Closed = false, false
}

func (s *chestSuite) carry(name string) *object.Instance {
	s.T().Helper()
	d := object.NewDefinition(name, name, "wrathrock", rules.ObjectCategoryOther, nil,
		"a "+name, "A "+name+" is here.", rules.SlotNone, rules.ArmorTypeNone, nil)
	inst := object.NewInstance(uuid.New(), d)
	s.Require().NoError(s.p.Inventory().Add(inst))
	return inst
}

func (s *chestSuite) TestPutIn() {
	s.open()
	pelt := s.carry("pelt")

	s.do(command.Put{Target: "pelt", Into: "box"})

	s.Assert().Equal(event.Put{Actor: "testdood", Item: "a pelt", Into: "a strongbox"}, sent[event.Put](s.T(), s.r, 0))
	_, inBox := s.box.Contents.Get(pelt.Id)
	s.Assert().True(inBox)
	_, carried := s.p.Inventory().Get(pelt.Id)
	s.Assert().False(carried)
	s.Assert().True(pelt.DecaysAt.IsZero(), "kept, not left lying")
}

func (s *chestSuite) TestPutAllSkipsWhatsWorn() {
	s.open()
	s.carry("pelt")
	s.carry("pelt")
	helm := object.NewDefinition("helm", "helm", "wrathrock", rules.ObjectCategoryArmor, nil,
		"a helm", "A helm is here.", rules.SlotHead, rules.ArmorTypeLeather, nil)
	worn := object.NewInstance(uuid.New(), helm)
	s.Require().NoError(s.p.Inventory().Add(worn))
	s.p.Equipment().Equip(rules.SlotHead, worn)

	s.do(command.Put{Target: "all", Into: "box"})
	s.Assert().Len(s.r.Sent, 2, "two pelts, and the helm stays on")

	s.do(command.Put{Target: "helm", Into: "box"})
	s.Assert().Equal(event.TargetInUse, s.failed())
}

func (s *chestSuite) TestPutCoins() {
	s.open()
	s.p.AddCoins(30)

	s.do(command.Put{Target: "20 coins", Into: "box"})
	s.Assert().Equal(20, s.box.Coins)
	s.Assert().Equal(10, s.p.Coins())

	s.do(command.Put{Target: "50 coins", Into: "box"})
	s.Assert().Equal(event.NotEnoughCoins, s.failed())

	s.do(command.Put{Target: "coins", Into: "box"})
	s.Assert().Equal(30, s.box.Coins, "all of them")
	s.Assert().Zero(s.p.Coins())
}

func (s *chestSuite) TestPutRefusals() {
	s.carry("pelt")

	s.do(command.Put{Target: "pelt", Into: "box"})
	s.Assert().Equal(event.ContainerClosed, s.failed())
	s.do(command.Put{Target: "pelt"})
	s.Assert().Equal(event.NoContainer, s.failed())
	s.do(command.Put{Into: "box"})
	s.Assert().Equal(event.NoTarget, s.failed())
	s.open()
	s.do(command.Put{Target: "dragon", Into: "box"})
	s.Assert().Equal(event.TargetNotFound, s.failed())
}
