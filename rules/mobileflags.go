package rules

import "errors"

type MobileFlag string

const (
	MobileFlagNone            MobileFlag = ""
	MobileFlagAggressive      MobileFlag = "aggressive"
	MobileFlagPlayerCantFight MobileFlag = "nofight"
	// MobileFlagMoonstruck is aggressive on a full-moon night in Wrathrock's
	// sky, and not otherwise. See world/moon.go.
	MobileFlagMoonstruck MobileFlag = "moonstruck"
)

var ErrUnknownMobileFlag = errors.New("unknown mobile flag")

var mobileFlags = enum[MobileFlag]{
	"mobile flags",
	ErrUnknownMobileFlag,
	[]MobileFlag{
		MobileFlagNone,
		MobileFlagAggressive,
		MobileFlagPlayerCantFight,
		MobileFlagMoonstruck,
	},
}

func (f *MobileFlag) UnmarshalText(b []byte) error { return mobileFlags.unmarshal(f, b) }

func ParseMobileFlag(s string) (MobileFlag, error) { return mobileFlags.parse(s) }
