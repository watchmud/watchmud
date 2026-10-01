package telnet

import (
	"strings"
	"unicode/utf8"
)

// The window width comes from the client, over NAWS (RFC 1073): the server
// asks with DO NAWS, and a client that agrees sends its size, and again
// whenever the window changes. A client that never says gets no wrapping,
// which is what every client got before -- the bots included.
const (
	// minWrap is the narrowest window worth wrapping for. A client reporting
	// less is more likely confused than tiny, and wrapping at 5 columns would
	// make the game unreadable for the sake of a bad number.
	minWrap = 20
	// maxWrap is as wide as a line gets made: a line of prose stretched across
	// a 300-column window is no easier to read.
	maxWrap = 200
)

// windowSize is the client's window width, from readPump to writePump through
// the send queue, like endOfRecord. Zero means don't wrap.
type windowSize int

// wrapWidth is the width a reported one is wrapped to: zero for nonsense,
// and never wider than maxWrap.
func wrapWidth(reported int) int {
	if reported < minWrap {
		return 0
	}
	return min(reported, maxWrap)
}

// wrap breaks every line of text wider than width at spaces. A line that fits
// is left exactly as it is, so tables padded with spaces keep their columns.
// A line's leading spaces indent its continuations too, so a room
// description stays a block. Color doesn't take up room, and a word wider
// than the window gets a line to itself rather than being cut.
func wrap(text string, width int) string {
	if width <= 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if visibleWidth(l) > width {
			lines[i] = wrapLine(l, width)
		}
	}
	return strings.Join(lines, "\n")
}

func wrapLine(line string, width int) string {
	body := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(body)]
	if len(indent) >= width/2 {
		indent = "" // an indent that eats the window helps nobody
	}

	var b strings.Builder
	b.WriteString(indent)
	col := len(indent)
	first := true
	for _, word := range strings.Fields(body) {
		w := visibleWidth(word)
		if !first && col+1+w > width {
			b.WriteString("\n" + indent)
			col = len(indent)
			first = true
		}
		if !first {
			b.WriteByte(' ')
			col++
		}
		b.WriteString(word)
		col += w
		first = false
	}
	return b.String()
}

// visibleWidth is how many columns s takes on screen: its runes, less the
// color codes, which take none.
func visibleWidth(s string) int {
	return utf8.RuneCountInString(plain(s))
}
