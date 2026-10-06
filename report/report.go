// Package report is what a player files with bug, idea or typo: a note for
// whoever runs the game, with where they were standing when they wrote it.
// A leaf, so the world and a store can both name it.
package report

import "time"

// MaxLength bounds one report: a note, not an essay, and never a flood.
const MaxLength = 500

type Report struct {
	Kind   string // "bug", "idea" or "typo"
	Player string
	Room   string // "zone/room"
	Text   string
	At     time.Time
}
