package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

// testcontent's kit backfills the temple token. The suite's player is one
// from before it: they have received nothing.
type backfillSuite struct {
	worldTestSuite
}

func TestBackfillSuite(t *testing.T) {
	suite.Run(t, new(backfillSuite))
}

func (s *backfillSuite) token() *object.Instance {
	for item := range s.p.Inventory().All() {
		if item.Definition.ObjectId.DefinitionId == "temple_token" {
			return item
		}
	}
	return nil
}

// received is how many Received events the player was sent
func (s *backfillSuite) received() int {
	n := 0
	for _, m := range s.r.Sent {
		if _, ok := m.(event.Received); ok {
			n++
		}
	}
	return n
}

func (s *backfillSuite) tokens() int {
	n := 0
	for item := range s.p.Inventory().All() {
		if item.Definition.ObjectId.DefinitionId == "temple_token" {
			n++
		}
	}
	return n
}

// neck free: worn, and told so -- after the room, which is what they asked for
func (s *backfillSuite) TestWornWhenTheSlotIsFree() {
	s.w.Arrive(s.p)

	token := s.token()
	s.Require().NotNil(token)
	s.Assert().Same(token, s.p.Equipment().At(rules.EquipmentSlot("neck")))
	s.Assert().Equal(1, token.Power, "made at the kit's power")
	// after the color setting and the room
	s.Assert().Equal(event.Received{Item: "a temple token", Worn: true}, sent[event.Received](s.T(), s.r, 3))
	s.Assert().True(s.p.HasBackfilled("wrathrock/temple_token"))

	s.r.Clear()
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Recall{})))
	s.Assert().Same(s.w.StartRoom, s.w.playerRoom(s.p), "and recall works")
}

// Review focus 1: the neck is taken -- in the pack, told to wear it, and
// recall waits until they do
func (s *backfillSuite) TestPackedWhenTheSlotIsTaken() {
	charm := object.NewInstance(uuid.New(), object.NewTestDefinition(s.T(), "charm",
		rules.EquipmentSlot("neck"), rules.ObjectCategoryOther, rules.ArmorTypeNone))
	s.Require().NoError(s.p.Inventory().Add(charm))
	s.p.Equipment().Equip(rules.EquipmentSlot("neck"), charm)

	s.w.Arrive(s.p)

	s.Require().NotNil(s.token())
	s.Assert().Same(charm, s.p.Equipment().At(rules.EquipmentSlot("neck")), "the charm stays on")
	s.Assert().Equal(event.Received{Item: "a temple token", Worn: false}, sent[event.Received](s.T(), s.r, 3))

	s.r.Clear()
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Recall{})))
	s.Assert().Equal(event.NotGranted, sent[event.Failed](s.T(), s.r, 0).Code)
}

// Review focus 2: once, however many logins
func (s *backfillSuite) TestOnce() {
	s.w.Arrive(s.p)
	s.r.Clear()

	s.w.Arrive(s.p)

	s.Assert().Equal(1, s.tokens())
	s.Assert().Zero(s.received())
}

// and once across a save: a record that has it doesn't get it again
func (s *backfillSuite) TestOnceAcrossASave() {
	s.w.Arrive(s.p)
	rec := s.w.record(s.p)
	s.w.RemovePlayer(s.p)

	back, err := player.FromRecord(rec, s.r, s.w.content.Catalog, s.w)
	s.Require().NoError(err)
	s.w.ReturnPlayer(back, rec.LastZoneId, rec.LastRoomId)
	s.r.Clear()
	s.w.Arrive(back)

	n := 0
	for item := range back.Inventory().All() {
		if item.Definition.ObjectId.DefinitionId == "temple_token" {
			n++
		}
	}
	s.Assert().Equal(1, n)
}

// Review focus 3: a new character got it at creation, and gets nothing more
func (s *backfillSuite) TestNewCharacterGetsNothingMore() {
	fresh := player.NewTestPlayer(uuid.New(), "newbie", s.r)
	player.GiveStartingGear(fresh, s.w.content.Catalog.StartingGear, s.w)
	s.w.PlacePlayer(fresh, s.w.StartRoom)
	s.r.Clear()

	s.w.Arrive(fresh)

	s.Assert().Zero(s.received())
}
