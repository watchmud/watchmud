package world

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/rules"
)

type handleSellSuite struct{ tradeSuite }

func TestHandleSellSuite(t *testing.T) { suite.Run(t, new(handleSellSuite)) }

// 40% of the price, and gone.
func (s *handleSellSuite) TestSell() {
	s.toStore()
	s.carry("cap", 2, 20, 20) // 40, as new

	s.do(command.Sell{Target: "cap"})

	s.Assert().Equal(event.Sold{Item: "cap", Coins: 16}, sent[event.Sold](s.T(), s.r, 0))
	s.Assert().Equal(16, s.p.Coins())
	s.Assert().Zero(s.p.Inventory().Len())
}

// Worn sells for less, and broken for nothing: that's the smith's business.
func (s *handleSellSuite) TestSell_wearCostsYou() {
	s.toStore()
	s.carry("cap", 2, 10, 20)
	s.carry("boots", 2, 0, 20)

	s.do(command.Sell{Target: "cap"})
	s.Assert().Equal(8, sent[event.Sold](s.T(), s.r, 0).Coins, "half worn, half the money")

	s.do(command.Sell{Target: "boots"})
	s.Assert().Equal(event.Worthless, sent[event.Failed](s.T(), s.r, 1).Code)
	s.Assert().Equal(1, s.p.Inventory().Len(), "the boots are still yours")
}

// Like drop: not what you have on.
func (s *handleSellSuite) TestSell_notWhatYouAreWearing() {
	s.toStore()
	worn := s.carry("cap", 2, 20, 20)
	s.p.Equipment().Equip(rules.SlotHead, worn)

	s.do(command.Sell{Target: "cap"})

	s.Assert().Equal(event.TargetInUse, sent[event.Failed](s.T(), s.r, 0).Code)
	s.Assert().Zero(s.p.Coins())
}

// sell all.<thing> sells what it can and passes over the rest quietly.
func (s *handleSellSuite) TestSell_all() {
	s.toStore()
	worn := s.carry("cap", 2, 20, 20)
	s.p.Equipment().Equip(rules.SlotHead, worn)
	s.carry("cap", 1, 20, 20) // 20: 8
	s.carry("cap", 1, 0, 20)  // broken: nothing

	s.do(command.Sell{Target: "all.cap"})

	s.Require().Len(s.r.Sent, 1)
	s.Assert().Equal(8, s.p.Coins())
	s.Assert().Equal(2, s.p.Inventory().Len(), "the one worn and the broken one")
}

func (s *handleSellSuite) TestSell_notCarried() {
	s.toStore()
	s.do(command.Sell{Target: "cap"})
	s.Assert().Equal(event.TargetNotFound, sent[event.Failed](s.T(), s.r, 0).Code)
}

// No sequence of buying, selling and repairing can make coins: buy at 40,
// sell back at 16.
func (s *handleSellSuite) TestSell_noProfitInBuyingAndSellingBack() {
	s.toStore()
	s.p.AddCoins(40)

	s.do(command.Buy{Target: "helmet"})
	s.do(command.Sell{Target: "helmet"})

	s.Assert().Equal(16, s.p.Coins())
}
