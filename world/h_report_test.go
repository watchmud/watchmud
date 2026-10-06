package world

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/report"
)

type reportSuite struct{ worldTestSuite }

func TestReportSuite(t *testing.T) { suite.Run(t, new(reportSuite)) }

func (s *reportSuite) do(cmd command.Command) {
	s.T().Helper()
	s.r.Clear()
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(cmd)))
}

// filed with who and where, thanked, and readable by a wizard newest first
func (s *reportSuite) TestFiled() {
	var filed []report.Report
	s.w.SetReportFiler(func(r report.Report) { filed = append(filed, r) })

	s.do(command.Report{Kind: "typo", Text: "'recieve' in the market"})
	s.do(command.Report{Kind: "idea", Text: "a fishing pond"})

	s.Assert().Equal(event.Reported{Kind: "idea"}, sent[event.Reported](s.T(), s.r, 0))
	s.Require().Len(filed, 2)
	s.Assert().Equal("testdood", filed[0].Player)
	s.Assert().Equal("wrathrock/temple_square", filed[0].Room)
	s.Assert().False(filed[0].At.IsZero())

	s.p.SetWizard(true)
	s.do(command.Reports{})
	list := sent[event.ReportList](s.T(), s.r, 0).Reports
	s.Require().Len(list, 2)
	s.Assert().Equal("idea", list[0].Kind, "newest first")
}

func (s *reportSuite) TestRefusals() {
	s.do(command.Report{Kind: "bug"})
	s.Assert().Equal(event.NoValue, sent[event.Failed](s.T(), s.r, 0).Code)
	s.do(command.Report{Kind: "bug", Text: strings.Repeat("x", report.MaxLength+1)})
	s.Assert().Equal(event.TooLong, sent[event.Failed](s.T(), s.r, 0).Code)
}

// it keeps the latest, not everything since the start
func (s *reportSuite) TestKeepsTheLatest() {
	for range keptReports + 5 {
		s.do(command.Report{Kind: "bug", Text: "x"})
	}
	s.Assert().Len(s.w.reports, keptReports)
}
