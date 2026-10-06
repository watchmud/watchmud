package world

import (
	"time"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/report"
)

// keptReports is how many reports the world holds for "reports": the latest,
// since the last restart. The log and the store keep every one.
const keptReports = 50

// SetReportFiler is what keeps a report beyond the log and the latest few in
// memory -- mongo, in the server. It's called on the world goroutine, so it
// mustn't block: hand the report to a goroutine of its own.
func (w *World) SetReportFiler(f func(report.Report)) { w.fileReport = f }

// handleReport is bug, idea and typo: a note for whoever runs the game, with
// the player and where they stood. It's logged, kept for "reports", and
// filed if the server has somewhere to file it.
func (w *World) handleReport(msg *gameserver.HandlerParameter, cmd command.Report) {
	if cmd.Text == "" {
		msg.Fail(event.NoValue)
		return
	}
	if len(cmd.Text) > report.MaxLength {
		msg.Fail(event.TooLong)
		return
	}
	r := report.Report{Kind: cmd.Kind, Player: msg.Player.Name(), Text: cmd.Text, At: time.Now().UTC()}
	if room := w.playerRoom(msg.Player); room != nil {
		r.Room = room.Zone.Id + "/" + room.Id
	}
	log.Warn().Str("report", r.Kind).Str("player", r.Player).Str("room", r.Room).Msg(r.Text)
	w.reports = append(w.reports, r)
	if len(w.reports) > keptReports {
		w.reports = w.reports[len(w.reports)-keptReports:]
	}
	if w.fileReport != nil {
		w.fileReport(r)
	}
	msg.Player.Send(event.Reported{Kind: r.Kind})
}

// handleReports is a wizard reading the latest reports, newest first.
func (w *World) handleReports(msg *gameserver.HandlerParameter, cmd command.Reports) {
	var list event.ReportList
	for i := len(w.reports) - 1; i >= 0; i-- {
		r := w.reports[i]
		list.Reports = append(list.Reports, event.ReportEntry{
			Kind: r.Kind, Player: r.Player, Room: r.Room, Text: r.Text, When: r.At.Format("Jan 2 15:04 UTC"),
		})
	}
	msg.Player.Send(list)
}
