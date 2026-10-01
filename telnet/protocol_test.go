package telnet

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIACFilter(t *testing.T) {

	cases := []struct {
		name string
		in   []byte
		want string
	}{
		{"plain text", []byte("look\r\n"), "look\r\n"},
		{"nul dropped", []byte("look\r\x00"), "look\r"},
		{"WILL stripped", []byte{IAC, WILL, optEcho, 'l', 'o', 'o', 'k'}, "look"},
		{"DO stripped", append([]byte{IAC, DO, optNAWS}, "look"...), "look"},
		{"WONT and DONT stripped", []byte{IAC, WONT, optEcho, IAC, DONT, optNAWS, 'h', 'i'}, "hi"},
		{"escaped IAC survives", []byte{'a', IAC, IAC, 'b'}, "a\xffb"},
		{"single-byte command stripped", []byte{IAC, NOP, 'h', 'i'}, "hi"},
		{"subnegotiation stripped",
			append([]byte{IAC, SB, optNAWS, 0, 80, 0, 24, IAC, SE}, "look"...), "look"},
		{"subnegotiation containing escaped IAC",
			append([]byte{IAC, SB, optNAWS, IAC, IAC, 0, IAC, SE}, "hi"...), "hi"},
		{"negotiation only", []byte{IAC, WILL, optEcho}, ""},
		{"empty input", []byte{}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &iacFilter{src: bufio.NewReader(bytes.NewReader(tc.in))}
			got, err := io.ReadAll(f)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(got))
		})
	}
}

// A client's negotiation can straddle any Read boundary. The filter's state
// lives on the struct precisely so that works; this proves it.
func TestIACFilterAcrossReads(t *testing.T) {
	in := append([]byte{IAC, WILL, 1}, "look"...)
	in = append(in, IAC, SB, 31, 0, 80, IAC, SE)
	in = append(in, "\r\n"...)

	f := &iacFilter{src: bufio.NewReader(bytes.NewReader(in))}
	var got []byte
	buf := make([]byte, 1) // one byte at a time: every state transition straddles a call
	for {
		n, err := f.Read(buf)
		got = append(got, buf[:n]...)
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
	}
	assert.Equal(t, "look\r\n", string(got))
}

func TestIACFilterWithScanner(t *testing.T) {
	in := append([]byte{IAC, WILL, 1}, "look\r\n"...)
	in = append(in, IAC, SB, 31, 0, 80, 0, 24, IAC, SE)
	in = append(in, "north\r\n"...)

	s := bufio.NewScanner(&iacFilter{src: bufio.NewReader(bytes.NewReader(in))})
	var lines []string
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	require.NoError(t, s.Err())
	assert.Equal(t, []string{"look", "north"}, lines)
}

func TestIACFilter_reportsNegotiation(t *testing.T) {
	type heard struct{ verb, option byte }
	var got []heard
	in := []byte{IAC, DO, optEOR, 'h', 'i', IAC, DONT, optEcho, IAC, SB, optNAWS, 0, 80, 0, 24, IAC, SE}
	f := &iacFilter{
		src:        bufio.NewReader(bytes.NewReader(in)),
		negotiated: func(verb, option byte) { got = append(got, heard{verb, option}) },
	}

	text, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "hi", string(text))
	assert.Equal(t, []heard{{DO, optEOR}, {DONT, optEcho}}, got, "a subnegotiation isn't a negotiation")
}

func TestIACFilter_reportsSubnegotiation(t *testing.T) {
	type heard struct {
		option byte
		data   []byte
	}
	var got []heard
	// 0x01FF wide: the 0xFF is doubled on the wire
	in := []byte{'h', IAC, SB, optNAWS, 0x01, IAC, IAC, 0, 24, IAC, SE, 'i'}
	f := &iacFilter{
		src: bufio.NewReader(bytes.NewReader(in)),
		subnegotiated: func(option byte, data []byte) {
			got = append(got, heard{option, append([]byte(nil), data...)})
		},
	}

	text, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "hi", string(text))
	assert.Equal(t, []heard{{optNAWS, []byte{0x01, 0xFF, 0, 24}}}, got)
}

// A client can't make the filter hold more than maxSubneg of a
// subnegotiation, however long it goes on.
func TestIACFilter_boundsASubnegotiation(t *testing.T) {
	in := append([]byte{IAC, SB, 99}, bytes.Repeat([]byte{'x'}, 10000)...)
	in = append(in, IAC, SE)
	var size int
	f := &iacFilter{
		src:           bufio.NewReader(bytes.NewReader(in)),
		subnegotiated: func(_ byte, data []byte) { size = len(data) },
	}

	_, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, maxSubneg-1, size)
}
