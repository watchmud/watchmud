package telnet

import (
	"strings"

	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/rules"
)

// renderMap draws "map": each room a [ ] on the zone's grid, four columns
// and two rows apart, joined by - and | where there's a way through, # where
// a door is shut. You are the *; a room with a way up or down shows ^, v or
// both as +. North is up the screen.
//
// Under a screen reader this is the wrong answer -- a picture read out
// character by character -- and it will need a list in words instead.
func renderMap(m event.Map) string {
	if len(m.Rooms) == 0 {
		return "You can't make out a map of this place.\n"
	}
	minX, maxX, minY, maxY := 0, 0, 0, 0
	for _, r := range m.Rooms {
		minX, maxX = min(minX, r.X), max(maxX, r.X)
		minY, maxY = min(minY, r.Y), max(maxY, r.Y)
	}
	// a border of one cell each side, for the ways out of the edge rooms
	width := (maxX-minX)*4 + 5
	height := (maxY-minY)*2 + 3
	grid := make([][]rune, height)
	for i := range grid {
		grid[i] = []rune(strings.Repeat(" ", width))
	}
	set := func(row, col int, c rune) {
		if row >= 0 && row < height && col >= 0 && col < width {
			grid[row][col] = c
		}
	}
	var you event.MapRoom
	var marks, shut bool
	for _, r := range m.Rooms {
		row, col := (maxY-r.Y)*2+1, (r.X-minX)*4+1
		mark := ' '
		up, down := false, false
		for _, ex := range r.Exits {
			c := '-'
			if ex.Direction == rules.DirectionNorth || ex.Direction == rules.DirectionSouth {
				c = '|'
			}
			if ex.Closed {
				c, shut = '#', true
			}
			switch ex.Direction {
			case rules.DirectionNorth:
				set(row-1, col+1, c)
			case rules.DirectionSouth:
				set(row+1, col+1, c)
			case rules.DirectionEast:
				set(row, col+3, c)
			case rules.DirectionWest:
				set(row, col-1, c)
			case rules.DirectionUp:
				up = true
			case rules.DirectionDown:
				down = true
			}
		}
		switch {
		case r.Here:
			mark = '*'
			you = r
		case up && down:
			mark, marks = '+', true
		case up:
			mark, marks = '^', true
		case down:
			mark, marks = 'v', true
		}
		set(row, col, '[')
		set(row, col+1, mark)
		set(row, col+2, ']')
	}
	var b strings.Builder
	b.WriteString(paint(colorHeading, m.Zone) + "\n")
	// blank rows inside keep the distances true; only the border's go
	var lines []string
	for _, line := range grid {
		lines = append(lines, strings.TrimRight(string(line), " "))
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		b.WriteString(line + "\n")
	}
	b.WriteString("You are at the * (" + you.Name + ").")
	var updown []string
	for _, ex := range you.Exits {
		if ex.Direction == rules.DirectionUp || ex.Direction == rules.DirectionDown {
			updown = append(updown, strings.ToLower(ex.Direction.String())+" to "+ex.Leads)
		}
	}
	if len(updown) > 0 {
		b.WriteString(" From here: " + strings.Join(updown, ", ") + ".")
	}
	b.WriteString("\n")
	var legend []string
	if marks {
		legend = append(legend, "^ v + a way up, down or both")
	}
	if shut {
		legend = append(legend, "# a closed door")
	}
	if len(legend) > 0 {
		b.WriteString(strings.Join(legend, "; ") + ".\n")
	}
	return b.String()
}
