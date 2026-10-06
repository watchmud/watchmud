package mongostore

import (
	"context"
	"fmt"
	"time"

	"github.com/watchmud/watchmud/report"
)

// reportDoc is a report as the collection holds it: readable as it stands,
// for whoever runs the game.
type reportDoc struct {
	Kind   string    `bson:"kind"`
	Player string    `bson:"player"`
	Room   string    `bson:"room"`
	Text   string    `bson:"text"`
	At     time.Time `bson:"at"`
}

// File keeps one report. It blocks for as long as mongo takes, up to the
// store's timeout: the server calls it on a goroutine of its own.
func (s *Store) File(r report.Report) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if _, err := s.reports.InsertOne(ctx, reportDoc(r)); err != nil {
		return fmt.Errorf("mongostore: filing a %s report from %s: %w", r.Kind, r.Player, err)
	}
	return nil
}
