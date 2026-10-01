package world

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
)

type handleValueSuite struct{ tradeSuite }

func TestHandleValueSuite(t *testing.T) { suite.Run(t, new(handleValueSuite)) }

func (s *handleValueSuite) TestValue() {
	s.toStore()
	s.carry("cap", 2, 20, 20)

	s.do(command.Value{Target: "cap"})

	s.Assert().Equal(event.Valued{Item: "cap", Coins: 16}, sent[event.Valued](s.T(), s.r, 0))
	s.Assert().Equal(1, s.p.Inventory().Len(), "asking isn't selling")
	s.Assert().Zero(s.p.Coins())
}

func (s *handleValueSuite) TestValue_broken() {
	s.toStore()
	s.carry("cap", 2, 0, 20)
	s.do(command.Value{Target: "cap"})
	s.Assert().Equal(event.Worthless, sent[event.Failed](s.T(), s.r, 0).Code)
}
