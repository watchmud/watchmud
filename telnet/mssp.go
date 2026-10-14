package telnet

import (
	"strconv"
	"strings"
	"time"
)

// MSSP, the MUD Server Status Protocol: a crawler from a listing site
// connects, answers the server's WILL MSSP with DO, and is told the game's
// name, how many are playing and since when, and what it is -- as variable
// and value pairs in one subnegotiation. Players' clients ignore it.
const (
	optMSSP = 70
	msspVar = 1
	msspVal = 2
)

// MSSP is what the game says about itself. Players and Started are read
// fresh for every crawler; the rest is fixed for the server's life.
type MSSP struct {
	// Hostname and Website are where to find it; empty is left unsaid.
	Hostname, Website string
	// Contact is an address for whoever runs it; empty is left unsaid.
	Contact string
	// Port is plain telnet's, TLSPort the encrypted one's (0: none).
	Port, TLSPort int
	// Players is how many people are playing now, bots not counted.
	Players func() int
	// Started is when the server came up: MSSP's UPTIME.
	Started time.Time
}

// msspFixed is what's true of the game wherever it runs, in the order a
// crawler is told it. Statements about play are kept true by hand: no
// classes and no levels (gear decides, LEVELS.md), no player killing.
var msspFixed = [][2]string{
	{"CODEBASE", "WatchMUD"},
	{"FAMILY", "Custom"},
	{"GENRE", "Fantasy"},
	{"SUBGENRE", "Medieval Fantasy"},
	{"GAMEPLAY", "Hack and Slash"},
	{"STATUS", "Open Beta"},
	{"LANGUAGE", "English"},
	{"LOCATION", "United States"},
	{"CREATED", "2026"},
	{"WORLD ORIGINALITY", "All Original"},
	{"PLAYERKILLING", "None"},
	{"CLASSES", "0"},
	{"LEVELS", "0"},
	{"ANSI", "1"},
	{"GMCP", "1"},
	{"UTF-8", "1"},
	{"MCCP", "1"},
	{"MSDP", "0"},
	{"MXP", "0"},
	{"PUEBLO", "0"},
	{"VT100", "0"},
	{"XTERM 256 COLORS", "0"},
}

// subnegotiation is the whole answer: IAC SB MSSP, the pairs, IAC SE.
func (m *MSSP) subnegotiation() []byte {
	b := []byte{IAC, SB, optMSSP}
	add := func(name, value string) {
		b = append(b, msspVar)
		b = append(b, msspSafe(name)...)
		b = append(b, msspVal)
		b = append(b, msspSafe(value)...)
	}
	players := 0
	if m.Players != nil {
		players = m.Players()
	}
	add("NAME", "WatchMUD")
	add("PLAYERS", strconv.Itoa(players))
	add("UPTIME", strconv.FormatInt(m.Started.Unix(), 10))
	if m.Hostname != "" {
		add("HOSTNAME", m.Hostname)
	}
	add("PORT", strconv.Itoa(m.Port))
	if m.TLSPort != 0 {
		add("SSL", strconv.Itoa(m.TLSPort))
	}
	if m.Website != "" {
		add("WEBSITE", m.Website)
	}
	if m.Contact != "" {
		add("CONTACT", m.Contact)
	}
	for _, kv := range msspFixed {
		add(kv[0], kv[1])
	}
	return append(b, IAC, SE)
}

// msspSafe drops what would end a name or value early: the separators, and
// IAC. Config is ASCII in practice; this keeps a typo from breaking the rest.
func msspSafe(s string) string {
	return strings.Map(func(r rune) rune {
		if r == msspVar || r == msspVal || r == 0 || r >= 0x80 {
			return -1
		}
		return r
	}, s)
}
