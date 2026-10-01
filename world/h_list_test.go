package world

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
)

type handleListSuite struct{ tradeSuite }

func TestHandleListSuite(t *testing.T) { suite.Run(t, new(handleListSuite)) }

func (s *handleListSuite) TestList() {
	s.toStore()
	s.do(command.List{})

	s.Assert().Equal(event.ShopList{Items: []event.ShopEntry{
		{Item: "iron helmet", Power: 2, Price: 40},
		{Item: "knife", Power: 1, Price: 20},
	}}, sent[event.ShopList](s.T(), s.r, 0))
}

func (s *handleListSuite) TestList_notAShop() {
	s.do(command.List{})
	s.Assert().Equal(event.NoShop, sent[event.Failed](s.T(), s.r, 0).Code)
}
