package telnet

import (
	"strings"
	"unicode/utf8"
)

// cell is one entry in a table: what it says, and the color it's painted.
// Right puts it against the right edge of its column, for numbers.
type cell struct {
	text  string
	color string
	right bool
}

func plainCell(s string) cell  { return cell{text: s} }
func colored(c, s string) cell { return cell{text: s, color: c} }
func number(s string) cell     { return cell{text: s, right: true} }

// table lays rows out in columns, each as wide as its widest cell, measured
// before color -- so it lines up with color on or off -- indented two spaces
// with two between columns. The last column isn't padded: nothing follows it.
// Rows may be short; a missing cell is an empty one.
func table(rows [][]cell) string {
	var widths []int
	for _, row := range rows {
		for i, c := range row {
			if i == len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], utf8.RuneCountInString(c.text)) // columns, not bytes
		}
	}
	var b strings.Builder
	for _, row := range rows {
		var line strings.Builder
		for i, c := range row {
			gap := strings.Repeat(" ", widths[i]-utf8.RuneCountInString(c.text))
			shown := c.text
			if c.color != "" {
				shown = paint(c.color, c.text)
			}
			if c.right {
				line.WriteString(gap + shown)
			} else if i < len(row)-1 {
				line.WriteString(shown + gap)
			} else {
				line.WriteString(shown)
			}
			if i < len(row)-1 {
				line.WriteString("  ")
			}
		}
		b.WriteString("  " + strings.TrimRight(line.String(), " ") + "\n")
	}
	return b.String()
}
