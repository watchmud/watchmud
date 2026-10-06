package world

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
)

// A sweeper in Market Square, and things lying on its floor.
type janitorSuite struct {
	worldTestSuite
	market *spaces.Room
	mob    *mobile.Instance
	now    time.Time
}

func TestJanitorSuite(t *testing.T) {
	suite.Run(t, new(janitorSuite))
}

func (s *janitorSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	var found bool
	s.market, found = s.w.findRoomById("wrathrock", "market_square")
	s.Require().True(found)
	d := mobile.NewDefinition("janitor", "janitor", "wrathrock", nil, "The janitor.", "The janitor sweeps.",
		10, rules.WanderDefinition{}, 10, false)
	s.mob = mobile.NewInstance(d)
	s.w.PlaceMobile(s.mob, s.market)
	s.w.movePlayerMagically(s.p, s.market)
	s.r.Clear()
	s.now = time.Now()
}

// lay puts something on room's floor that was dropped ago.
func (s *janitorSuite) lay(room *spaces.Room, name string, ago time.Duration, behaviors ...rules.ObjectBehavior) *object.Instance {
	s.T().Helper()
	d := object.NewDefinition(name, name, "wrathrock", rules.ObjectCategoryOther, nil,
		"a "+name, "A "+name+" is here.", rules.SlotNone, rules.ArmorTypeNone, behaviors)
	inst := object.NewInstance(uuid.New(), d)
	inst.DecaysAt = s.now.Add(-ago).Add(rules.DroppedDecay)
	s.Require().NoError(room.Inventory.Add(inst))
	return inst
}

func (s *janitorSuite) TestWhatIsJunk() {
	old := s.lay(s.market, "pelt", 10*time.Minute)
	s.lay(s.market, "knife", time.Minute) // just dropped: maybe for a friend
	s.lay(s.market, "corpse", 10*time.Minute, rules.ObjectBehaviorNoTake)
	reset := object.NewInstance(uuid.New(), old.Definition) // a zone reset's: never dropped
	s.Require().NoError(s.market.Inventory.Add(reset))

	s.Assert().Equal([]*object.Instance{old}, s.w.junk(s.market, s.now))
}

// a bag with things in it isn't junk: junk itself refuses that loss
func (s *janitorSuite) TestNotAFullBag() {
	bag := s.lay(s.market, "satchel", time.Hour)
	bag.Contents = object.NewList()
	empty := s.lay(s.market, "sack", time.Hour)
	empty.Contents = object.NewList()
	pelt := object.NewInstance(uuid.New(), bag.Definition)
	s.Require().NoError(bag.Contents.Add(pelt))

	s.Assert().Equal([]*object.Instance{empty}, s.w.junk(s.market, s.now))
}

// what's left in the donation room is left for someone
func (s *janitorSuite) TestNeverTheDonationRoom() {
	donation, found := s.w.findRoomById("wrathrock", "donation_room")
	s.Require().True(found)
	s.lay(donation, "pelt", time.Hour)
	s.Assert().Empty(s.w.junk(donation, s.now))
}

// oldest first, the room told of each, and only as many as asked
func (s *janitorSuite) TestSweep() {
	s.lay(s.market, "pelt", 20*time.Minute)
	s.lay(s.market, "bone", 10*time.Minute)
	s.lay(s.market, "hide", 15*time.Minute)

	s.Assert().Equal(3, s.w.junkHere(s.mob))
	s.Assert().Equal(2, s.w.sweep(s.mob, 2))

	s.Assert().Equal(event.Swept{Sweeper: "janitor", Item: "a pelt"}, sent[event.Swept](s.T(), s.r, 0))
	s.Assert().Equal(event.Swept{Sweeper: "janitor", Item: "a hide"}, sent[event.Swept](s.T(), s.r, 1))
	s.Assert().Equal(1, s.w.junkHere(s.mob), "the bone, last dropped, is left")
}
