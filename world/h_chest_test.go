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
