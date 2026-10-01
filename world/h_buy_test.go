package world

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
)

type handleBuySuite struct{ tradeSuite }

func TestHandleBuySuite(t *testing.T) { suite.Run(t, new(handleBuySuite)) }

func (s *handleBuySuite) TestBuy() {
	s.toStore()
	s.p.AddCoins(50)

	s.do(command.Buy{Target: "helmet"})

	s.Assert().Equal(event.Bought{Item: "iron helmet", Cost: 40}, sent[event.Bought](s.T(), s.r, 0))
	s.Assert().Equal(10, s.p.Coins())
	got := targetsIn(Target{Name: "helmet"}, s.p.Inventory().All())
	s.Require().Len(got, 1)
	s.Assert().Equal(2, got[0].Power, "at the shop's power")
}

func (s *handleBuySuite) TestBuy_cantAfford() {
	s.toStore()
	s.p.AddCoins(39)

	s.do(command.Buy{Target: "helmet"})

	s.Assert().Equal(event.TooExpensive{Item: "iron helmet", Cost: 40, Coins: 39}, sent[event.TooExpensive](s.T(), s.r, 0))
	s.Assert().Equal(39, s.p.Coins())
	s.Assert().Zero(s.p.Inventory().Len())
}

func (s *handleBuySuite) TestBuy_notForSale() {
	s.toStore()
	s.p.AddCoins(100)

	s.do(command.Buy{Target: "crown"})
	s.Assert().Equal(event.NotForSale, sent[event.Failed](s.T(), s.r, 0).Code)

	s.do(command.Buy{Target: "2.helmet"})
	s.Assert().Equal(event.NotForSale, sent[event.Failed](s.T(), s.r, 1).Code, "there's only the one kind")
	s.Assert().Equal(100, s.p.Coins())
}

func (s *handleBuySuite) TestBuy_notAShop() {
	s.p.AddCoins(100)
	s.do(command.Buy{Target: "helmet"})
	s.Assert().Equal(event.NoShop, sent[event.Failed](s.T(), s.r, 0).Code)
}
