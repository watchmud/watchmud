package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/lock"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
)

// A gate south of Temple Square, closed and locked, the same gate north of the
// market; bob waits in the market to hear it from the far side.
type doorSuite struct {
	worldTestSuite
	gate   *spaces.Door
	market *spaces.Room
	bob    *player.Player
	bobR   *player.Recorder
}

func TestDoorSuite(t *testing.T) {
	suite.Run(t, new(doorSuite))
}

func (s *doorSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.market = s.w.StartRoom.DestinationRoom(rules.DirectionSouth)
	s.gate = spaces.NewDoor("iron gate", []string{"bars"},
		lock.New(lock.State{Closed: true, Locked: true}, "wrathrock/gate_key"))
	s.w.StartRoom.SetDoor(rules.DirectionSouth, s.gate)
	s.market.SetDoor(rules.DirectionNorth, s.gate)
	s.bobR = &player.Recorder{}
	s.bob = player.NewTestPlayer(uuid.New(), "bob", s.bobR)
	s.w.PlacePlayer(s.bob, s.market)
}

func (s *doorSuite) carryKey() {
	s.T().Helper()
	d := object.NewDefinition("gate_key", "gate key", "wrathrock", rules.ObjectCategoryOther,
		[]string{"key"}, "an iron key", "An iron key is here.", rules.SlotNone, rules.ArmorTypeNone, nil)
	s.Require().NoError(s.p.Inventory().Add(object.NewInstance(uuid.New(), d)))
}

func (s *doorSuite) do(cmd command.Command) {
	s.T().Helper()
	s.r.Clear()
	s.bobR.Clear()
	s.w.HandleIncomingMessage(s.handlerParameter(cmd))
}

func (s *doorSuite) failed() event.ResultCode {
	s.T().Helper()
	return sent[event.Failed](s.T(), s.r, 0).Code
}

func (s *doorSuite) TestLockedShutsTheWay() {
	s.do(command.Move{Direction: rules.DirectionSouth})
	s.Assert().Equal(event.DoorShut, s.failed())

	s.do(command.Open{Target: "gate"})
	s.Assert().Equal(event.Locked, s.failed())
}

func (s *doorSuite) TestNoKeyNoUnlock() {
	s.do(command.Unlock{Target: "gate"})
	s.Assert().Equal(event.NoKey, s.failed())
	s.Assert().True(s.gate.Locked)
}

// unlock, open and through -- and bob, on the far side, hears the door
// without seeing who
func (s *doorSuite) TestUnlockOpenAndGo() {
	s.carryKey()

	s.do(command.Unlock{Target: "south"})
	s.Assert().Equal(event.DoorChanged{Actor: "testdood", Door: "iron gate", Direction: rules.DirectionSouth, Change: event.DoorUnlocked},
		sent[event.DoorChanged](s.T(), s.r, 0))
	s.Assert().Equal(event.DoorChanged{Door: "iron gate", Direction: rules.DirectionNorth, Change: event.DoorUnlocked},
		sent[event.DoorChanged](s.T(), s.bobR, 0))

	s.do(command.Open{Target: "bars"})
	s.Assert().False(s.gate.Closed)
	s.Assert().True(s.market.Passable(rules.DirectionNorth), "open from both sides")

	s.do(command.Move{Direction: rules.DirectionSouth})
	s.Assert().Equal(s.market, s.w.playerRoom(s.p))
}

func (s *doorSuite) TestLockingNeedsItClosed() {
	s.carryKey()
	s.gate.Locked, s.gate.Closed = false, false

	s.do(command.Lock{Target: "gate"})
	s.Assert().Equal(event.NotClosed, s.failed())

	s.do(command.Close{Target: "gate"})
	s.do(command.Lock{Target: "gate"})
	s.Assert().True(s.gate.Locked)
}

func (s *doorSuite) TestNoSuchDoor() {
	s.do(command.Open{Target: "east"})
	s.Assert().Equal(event.NoDoor, s.failed(), "an exit, but no door in it")
	s.do(command.Open{Target: "portcullis"})
	s.Assert().Equal(event.NoDoor, s.failed())
	s.do(command.Open{})
	s.Assert().Equal(event.NoTarget, s.failed())
}

// the exits list says which way is shut
func (s *doorSuite) TestExitsSayClosed() {
	s.do(command.Exits{})
	exits := sent[event.Exits](s.T(), s.r, 0).Exits
	for _, e := range exits {
		s.Assert().Equal(e.Direction == rules.DirectionSouth, e.Closed, e.Direction.String())
	}
}

// a zone reset shuts and locks it again
func (s *doorSuite) TestResetLocksItAgain() {
	s.gate.Locked, s.gate.Closed = false, false
	s.w.StartRoom.Zone.Doors = append(s.w.StartRoom.Zone.Doors, s.gate)

	s.w.StartRoom.Zone.Reset(s.w.occupancy)

	s.Assert().Equal(lock.State{Closed: true, Locked: true}, s.gate.State)
}
