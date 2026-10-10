package telnet

import "compress/zlib"

// MCCP2, the MUD Client Compression Protocol: the server offers WILL, a
// client that says DO is sent IAC SB MCCP2 IAC SE, and everything after it is
// one zlib stream, flushed at the end of every write so nothing waits on a
// block filling. Text is most of what a MUD sends and compresses several
// times over -- a phone on a poor connection notices.
const optMCCP2 = 86

// compress is the client's answer to WILL MCCP2, through the send queue:
// writePump owns the stream, and the switch has to land between two writes.
type compress bool

// compress starts or ends compression. Starting twice is ignored, as is
// ending what never started; ending finishes the stream, so the client sees
// it close and reads what follows as plain bytes again.
func (c *conn) compress(on bool) error {
	switch {
	case on && c.zout == nil:
		if err := c.writeRaw(string([]byte{IAC, SB, optMCCP2, IAC, SE})); err != nil {
			return err
		}
		c.zout = zlib.NewWriter(writerFunc(func(p []byte) (int, error) { return c.netConn.Write(p) }))
	case !on && c.zout != nil:
		z := c.zout
		c.zout = nil
		return z.Close()
	}
	return nil
}

// writerFunc lets the zlib stream write to whatever the socket is when it
// writes, rather than the one it was made with: a test swaps it.
type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }
