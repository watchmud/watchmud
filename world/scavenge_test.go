package world

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

// the janitor suite's room and mob, as a scavenger instead
type scavengeSuite struct{ janitorSuite }

func TestScavengeSuite(t *testing.T) { suite.Run(t, new(scavengeSuite)) }

func (s *scavengeSuite) TestTakesTheLongestLying() {
	s.lay(s.market, "pelt", 2*time.Minute)
	old := s.lay(s.market, "bone", 3*time.Minute)
	s.lay(s.market, "knife", 10*time.Second) // just dropped: a moment to pick it back up
	bag := s.lay(s.market, "satchel", time.Hour)
	bag.Contents = object.NewList()
	s.lay(s.market, "corpse", time.Hour, rules.ObjectBehaviorNoTake)

	s.Assert().Equal("a bone", s.w.mobTakes(s.mob))
	_, carried := s.mob.Inventory.Get(old.Id)
	s.Assert().True(carried)
	s.Assert().True(old.DecaysAt.IsZero(), "carried, not lying")
	s.Assert().Equal(event.Snatched{Mob: "janitor", Item: "a bone"}, sent[event.Snatched](s.T(), s.r, 0))

	s.Assert().Equal("a pelt", s.w.mobTakes(s.mob))
	s.Assert().Equal("", s.w.mobTakes(s.mob), "the knife's too new, the bag and the corpse never")
}

func (s *scavengeSuite) TestCarriesSoMuch() {
	for range MaxCarried + 1 {
		s.lay(s.market, "pelt", time.Hour)
	}
	for range MaxCarried {
		s.Require().NotEmpty(s.w.mobTakes(s.mob))
	}
	s.Assert().Equal("", s.w.mobTakes(s.mob))
	s.Assert().Equal(MaxCarried, s.mob.Inventory.Len())
}

// what it took is in its corpse
func (s *scavengeSuite) TestIntoTheCorpse() {
	pelt := s.lay(s.market, "pelt", time.Hour)
	s.Require().Equal("a pelt", s.w.mobTakes(s.mob))

	s.w.becomeCorpse(s.mob)

	var corpse *object.Instance
	for i := range s.market.Inventory.All() {
		if i.Contents != nil && i.Definition.NoTake() {
			corpse = i
		}
	}
	s.Require().NotNil(corpse)
	_, inside := corpse.Contents.Get(pelt.Id)
	s.Assert().True(inside)
}
