package rules

import "errors"

type MobileFlag string

const (
	MobileFlagNone            MobileFlag = ""
	MobileFlagAggressive      MobileFlag = "aggressive"
	MobileFlagPlayerCantFight MobileFlag = "nofight"
)

var ErrUnknownMobileFlag = errors.New("unknown mobile flag")

var mobileFlags = enum[MobileFlag]{
	"mobile flags",
	ErrUnknownMobileFlag,
	[]MobileFlag{
		MobileFlagNone,
		MobileFlagAggressive,
		MobileFlagPlayerCantFight,
	},
}

func (f *MobileFlag) UnmarshalText(b []byte) error { return mobileFlags.unmarshal(f, b) }

func ParseMobileFlag(s string) (MobileFlag, error) { return mobileFlags.parse(s) }
