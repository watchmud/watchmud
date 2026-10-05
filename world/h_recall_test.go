package world

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/object"
)

// recall in testcontent: no mana, 60s, not in a fight, a wizard's always.
// The player starts in the market square, a step from the start room.
type handleRecallSuite struct {
	worldTestSuite
	clock time.Time
}

func TestHandleRecallSuite(t *testing.T) {
	suite.Run(t, new(handleRecallSuite))
}

func (s *handleRecallSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.clock = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	s.w.now = func() time.Time { return s.clock }
	market, found := s.w.findRoomById("wrathrock", "market_square")
	s.Require().True(found)
	s.w.movePlayerMagically(s.p, market)
	s.r.Clear()
}

// wearToken puts testcontent's temple token on the player's neck
func (s *handleRecallSuite) wearToken() *object.Instance {
	s.T().Helper()
	d, found := s.w.ObjectDefinition("wrathrock", "temple_token")
	s.Require().True(found)
	token := object.NewInstance(uuid.New(), d)
	token.Power = 1
	s.Require().NoError(s.p.Inventory().Add(token))
	s.p.Equipment().Equip(d.EquipmentSlot, token)
	return token
}

func (s *handleRecallSuite) recall() {
	s.T().Helper()
	s.r.Clear()
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Recall{})))
}

func (s *handleRecallSuite) failed() event.ResultCode {
	s.T().Helper()
	return sent[event.Failed](s.T(), s.r, 0).Code
}

func (s *handleRecallSuite) TestTakesYouToTheStart() {
	s.wearToken()

	s.recall()

	s.Assert().Same(s.w.StartRoom, s.w.playerRoom(s.p))
	s.Assert().Equal(s.w.StartRoom.Name, sent[event.RoomDescription](s.T(), s.r, 0).Name)
	s.Assert().Equal(100, s.p.CurrentMana(), "no mana")
	s.Assert().Equal(s.clock.Add(60*time.Second), s.p.ReadyAt("recall"))
}

// no token, no recall -- and nothing moves
func (s *handleRecallSuite) TestNeedsTheToken() {
	market := s.w.playerRoom(s.p)

	s.recall()

	s.Assert().Equal(event.NotGranted, s.failed())
	s.Assert().Equal(event.Failed{Verb: "recall", Code: event.NotGranted}, sent[event.Failed](s.T(), s.r, 0))
	s.Assert().Same(market, s.w.playerRoom(s.p))
}

// taken off, it stops working: the gear is the truth
func (s *handleRecallSuite) TestTakenOff() {
	token := s.wearToken()
	s.p.Equipment().Unequip(token.Definition.EquipmentSlot)

	s.recall()

	s.Assert().Equal(event.NotGranted, s.failed())
}

func (s *handleRecallSuite) TestCooldown() {
	s.wearToken()
	s.recall()
	s.w.movePlayerMagically(s.p, s.w.Zone("wrathrock").Rooms["market_square"])

	s.clock = s.clock.Add(59 * time.Second)
	s.recall()
	s.Assert().Equal(event.NotReady, s.failed())

	s.clock = s.clock.Add(time.Second)
	s.recall()
	s.Assert().Same(s.w.StartRoom, s.w.playerRoom(s.p))
}

// a wizard needs no token and has no cooldown
func (s *handleRecallSuite) TestWizard() {
	s.p.SetWizard(true)

	s.recall()
	s.Assert().Same(s.w.StartRoom, s.w.playerRoom(s.p))
	s.Assert().True(s.p.ReadyAt("recall").IsZero(), "no cooldown started")

	s.w.movePlayerMagically(s.p, s.w.Zone("wrathrock").Rooms["market_square"])
	s.recall()
	s.Assert().Same(s.w.StartRoom, s.w.playerRoom(s.p), "and straight again")
}

// Recall used to be a free, certain way out of any fight -- which made flee,
// with its chance of failing, pointless. It is refused, the way walking off
// is, and refusing spends nothing.
func (s *handleRecallSuite) TestNotInAFight() {
	s.wearToken()
	drone, found := s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	s.Require().NoError(s.w.fightLedger.Fight(s.p, drone))
	market := s.w.playerRoom(s.p)

	s.recall()

	s.Assert().Same(market, s.w.playerRoom(s.p), "still where they were")
	s.Assert().Equal([]any{event.Failed{Verb: "recall", Code: event.InAFight}}, s.r.Sent)
	s.Assert().True(s.p.ReadyAt("recall").IsZero(), "no cooldown started")
}

// a wizard in a fight is still refused: wizards skip the gear, not the rule
func (s *handleRecallSuite) TestWizardNotInAFight() {
	s.p.SetWizard(true)
	drone, found := s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	s.Require().NoError(s.w.fightLedger.Fight(s.p, drone))

	s.recall()

	s.Assert().Equal(event.InAFight, s.failed())
}

// it's an ability like the others: cast recall works too
func (s *handleRecallSuite) TestCastRecall() {
	s.wearToken()
	cmd := command.Cast{Ability: "recall"}

	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(cmd)))

	s.Assert().Same(s.w.StartRoom, s.w.playerRoom(s.p))
}

func (s *handleRecallSuite) TestListedInAbilities() {
	s.wearToken()
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Abilities{})))

	var ids []string
	for _, g := range sent[event.Abilities](s.T(), s.r, 0).Granted {
		ids = append(ids, g.Name)
	}
	s.Assert().Contains(ids, "recall")
}
