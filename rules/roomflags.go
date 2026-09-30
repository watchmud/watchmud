package rules

import "errors"

type RoomFlag string

const (
	RoomFlagNone    RoomFlag = ""
	RoomFlagNoFight RoomFlag = "nofight"
)

var ErrUnknownRoomFlag = errors.New("unknown room flag")

var roomFlags = enum[RoomFlag]{
	"room flags",
	ErrUnknownRoomFlag,
	[]RoomFlag{
		RoomFlagNone,
		RoomFlagNoFight,
	},
}

func (f *RoomFlag) UnmarshalText(b []byte) error { return roomFlags.unmarshal(f, b) }

func ParseRoomFlag(s string) (RoomFlag, error) { return roomFlags.parse(s) }
