package world

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
)

type goldSuite struct {
	worldTestSuite
}

func TestGoldSuite(t *testing.T) {
	suite.Run(t, new(goldSuite))
}

func (s *goldSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.p.SetWizard(true)
}

func (s *goldSuite) gold(amount string) {
	s.T().Helper()
	s.r.Clear()
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Gold{Amount: amount})))
}

func (s *goldSuite) TestGivesCoins() {
	s.gold("500")
	s.gold("250")

	s.Assert().Equal(750, s.p.Coins())
	s.Assert().Equal(event.GoldGiven{Amount: 250, Coins: 750}, sent[event.GoldGiven](s.T(), s.r, 0))
}

func (s *goldSuite) TestBadAmounts() {
	for _, amount := range []string{"", "lots", "0", "-5", "1000001"} {
		s.gold(amount)
		s.Assert().Equal(event.BadRequest, sent[event.Failed](s.T(), s.r, 0).Code, "%q", amount)
	}
	s.Assert().Zero(s.p.Coins())
}

// a wizard command: anyone else is told it isn't a command at all
func (s *goldSuite) TestWizardsOnly() {
	s.p.SetWizard(false)

	s.gold("500")

	s.Assert().Zero(s.p.Coins())
}
