package world

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
)

type wizardSuite struct {
	worldTestSuite
}

func TestWizardSuite(t *testing.T) {
	suite.Run(t, new(wizardSuite))
}

// Every builder command. A new one needs the command.Wizard marker and a
// line here, or anyone can run it.
var wizardCommands = []command.Command{
	command.Load{Type: "mob", Id: "rabbit"},
	command.Restore{Target: "testdood"},
	command.RoomStatus{},
	command.NoHassle{},
	command.Slay{Target: "target"},
	command.Gold{Amount: "100"},
	command.Goto{Target: "wrathrock/market_square"},
	command.Transfer{Target: "testdood"},
	command.Purge{Target: "nothing"},
	command.ZReset{},
	command.Echo{Text: "hello"},
	command.Echo{Text: "hello", Global: true},
	command.Users{},
	command.Moderate{Target: "nobody"},
	command.Moderate{Target: "nobody", Freeze: true},
}

func (s *wizardSuite) TestTheListIsMarked() {
	for _, cmd := range wizardCommands {
		_, marked := cmd.(command.Wizard)
		s.Assert().True(marked, "%T is a builder command without the command.Wizard marker", cmd)
	}
}

// To anyone else a builder command doesn't exist: the same answer a verb
// nobody has heard of gets, and nothing happens.
func (s *wizardSuite) TestRefusedToPlayers() {
	rabbits := s.w.occupancy.MobileCount("rabbit")
	for _, cmd := range wizardCommands {
		s.r.Clear()
		s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(cmd)))

		s.Require().Len(s.r.Sent, 1, "%T", cmd)
		s.Assert().Equal(event.Failed{Verb: cmd.Verb(), Code: event.UnknownCommand}, s.r.Sent[0], "%T", cmd)
	}
	s.Assert().Equal(rabbits, s.w.occupancy.MobileCount("rabbit"), "no rabbit was loaded")
}

func (s *wizardSuite) TestAllowedToWizards() {
	s.p.SetWizard(true)
	rabbits := s.w.occupancy.MobileCount("rabbit")

	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Load{Type: "mob", Id: "rabbit"})))

	s.Assert().Equal(rabbits+1, s.w.occupancy.MobileCount("rabbit"))
}
