package telnet

import (
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

// TTYPE (RFC 1091) and MTTS, the MUD convention on top of it: asked
// repeatedly, a client answers its name, then its terminal type, then
// "MTTS <n>", a bitvector of what it can do -- and then the last answer
// again, which is how the asking knows to stop. Only one bit is read.
const (
	optTTYPE  = 24
	ttypeIs   = 0
	ttypeSend = 1

	// mttsScreenReader is the client saying its player uses a screen reader.
	mttsScreenReader = 64
	// maxTTypes is how many times to ask: the three MTTS defines, and no
	// more for a client that never repeats itself.
	maxTTypes = 3
)

// screenReaderClient goes through the send queue when a client's MTTS says
// it reads to a screen reader: writePump owns the rendering decision.
type screenReaderClient struct{}

// askTerminalType asks the client what it is, once more.
func (c *conn) askTerminalType() {
	c.Send(negotiation([]byte{IAC, SB, optTTYPE, ttypeSend, IAC, SE}))
}

// terminalType hears one answer, on readPump, and asks again until the
// client repeats itself or has answered enough.
func (c *conn) terminalType(answer string) {
	if n := len(c.ttypes); n >= maxTTypes || n > 0 && c.ttypes[n-1] == answer {
		return // the end of the cycle, or a client answering what nobody asked
	}
	c.ttypes = append(c.ttypes, answer)
	if len(c.ttypes) == 1 {
		log.Info().Msgf("telnet %s: client %q", c.netConn.RemoteAddr(), answer)
	}
	if bits, ok := mtts(answer); ok {
		if bits&mttsScreenReader != 0 {
			c.Send(screenReaderClient{})
		}
		return
	}
	if len(c.ttypes) < maxTTypes {
		c.askTerminalType()
	}
}

// mtts reads "MTTS 137" as its bits.
func mtts(answer string) (int, bool) {
	rest, ok := strings.CutPrefix(strings.ToUpper(answer), "MTTS ")
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(rest))
	return n, err == nil
}
