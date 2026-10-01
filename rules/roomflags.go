package rules

import "errors"

type RoomFlag string

const (
	RoomFlagNone    RoomFlag = ""
	RoomFlagNoFight RoomFlag = "nofight"
	// RoomFlagSmithy is where gear is repaired.
	RoomFlagSmithy RoomFlag = "smithy"
)

var ErrUnknownRoomFlag = errors.New("unknown room flag")

var roomFlags = enum[RoomFlag]{
	"room flags",
	ErrUnknownRoomFlag,
	[]RoomFlag{
		RoomFlagNone,
		RoomFlagNoFight,
		RoomFlagSmithy,
	},
}

func (f *RoomFlag) UnmarshalText(b []byte) error { return roomFlags.unmarshal(f, b) }

func ParseRoomFlag(s string) (RoomFlag, error) { return roomFlags.parse(s) }
