package telnet

import "bufio"

// Telnet protocol bytes (RFC 854).
const (
	EOR  = 239 // end of record (RFC 885), once the client agrees to it
	SE   = 240 // end of subnegotiation
	NOP  = 241
	GA   = 249 // go ahead: the prompt ends here
	SB   = 250 // begin subnegotiation
	WILL = 251
	WONT = 252
	DO   = 253
	DONT = 254
	IAC  = 255 // interpret as command
)

// options
const (
	optEcho = 1
	optEOR  = 25
	optNAWS = 31
)

// iacFilter parser states.
type filterState int

const (
	stateData      filterState = iota // passing bytes through
	stateIAC                          // saw IAC, next byte is the command
	stateOption                       // saw WILL/WONT/DO/DONT, consume the option byte
	stateSubneg                       // inside SB ... SE, collecting it
	stateSubnegIAC                    // saw IAC inside a subnegotiation
)

type iacFilter struct {
	src   *bufio.Reader
	state filterState
	verb  byte // the WILL/WONT/DO/DONT whose option byte comes next

	// negotiated, if set, hears every WILL/WONT/DO/DONT the client sends.
	// It runs on the reading goroutine.
	negotiated func(verb, option byte)

	// subnegotiated, if set, hears every SB ... SE: the option, then what
	// came with it, IAC IAC already undoubled. Also on the reading goroutine.
	subnegotiated func(option byte, data []byte)
	sub           []byte // the subnegotiation so far, option first
}

// maxSubneg bounds what a subnegotiation may carry before the rest is
// dropped: NAWS is four bytes, and nothing the server listens for is long.
const maxSubneg = 64

func (f *iacFilter) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		if n > 0 && f.src.Buffered() == 0 {
			break // we have data, don't block waiting for more
		}
		b, err := f.src.ReadByte()
		if err != nil {
			return n, err
		}
		switch f.state {
		case stateData:
			if b == IAC {
				f.state = stateIAC
			} else if b != 0 { // drop NUL from CR NUL
				p[n] = b
				n++
			}
		case stateIAC:
			switch {
			case b == IAC: // escaped literal 0xFF
				p[n] = b
				n++
				f.state = stateData
			case b >= WILL && b <= DONT:
				f.verb = b
				f.state = stateOption // one option byte follows
			case b == SB:
				f.sub = f.sub[:0]
				f.state = stateSubneg
			default: // NOP, GA, AYT, etc: single byte, nothing follows
				f.state = stateData
			}
		case stateOption:
			f.state = stateData
			if f.negotiated != nil {
				f.negotiated(f.verb, b)
			}
		case stateSubneg:
			if b == IAC {
				f.state = stateSubnegIAC
			} else {
				f.collect(b)
			}
		case stateSubnegIAC:
			switch b {
			case SE:
				f.state = stateData
				if f.subnegotiated != nil && len(f.sub) > 0 {
					f.subnegotiated(f.sub[0], f.sub[1:])
				}
			case IAC:
				f.collect(IAC) // a doubled IAC is a literal 0xFF in the data
				f.state = stateSubneg
			default:
				f.state = stateSubneg
			}
		}
	}
	return n, nil
}

func (f *iacFilter) collect(b byte) {
	if len(f.sub) < maxSubneg {
		f.sub = append(f.sub, b)
	}
}
