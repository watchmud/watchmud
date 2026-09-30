package bot

// The telnet bytes the bot has to step over. The server only ever sends
// WILL/WONT ECHO around a password, but anything in the IAC grammar is
// skipped, subnegotiation included, and a doubled IAC is a literal 0xFF.
const (
	se   = 240
	sb   = 250
	will = 251
	dont = 254
	iac  = 255
)

type iacState int

const (
	textState iacState = iota
	iacSeen
	optionState
	subnegState
	subnegIACState
)

// iacStripper removes telnet commands from a byte stream. It keeps its state
// between calls, since a sequence can be split across two reads.
type iacStripper struct {
	state iacState
}

func (s *iacStripper) strip(in []byte) []byte {
	out := make([]byte, 0, len(in))
	for _, b := range in {
		switch s.state {
		case textState:
			if b == iac {
				s.state = iacSeen
			} else {
				out = append(out, b)
			}
		case iacSeen:
			switch {
			case b == iac:
				out = append(out, b)
				s.state = textState
			case b >= will && b <= dont:
				s.state = optionState
			case b == sb:
				s.state = subnegState
			default:
				s.state = textState
			}
		case optionState:
			s.state = textState
		case subnegState:
			if b == iac {
				s.state = subnegIACState
			}
		case subnegIACState:
			if b == se {
				s.state = textState
			} else {
				s.state = subnegState
			}
		}
	}
	return out
}
