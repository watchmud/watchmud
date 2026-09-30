package rules

import "errors"

type ZoneReset string

const (
	ZoneResetNever     ZoneReset = ""          // Don't ever reset. Obviously.
	ZoneResetNoPlayers ZoneReset = "noPlayers" // Reset possible only when the zone is empty of players.
	ZoneResetAlways    ZoneReset = "always"    // Reset at the correct time, no matter what else is happening.
)

var ErrUnknownZoneReset = errors.New("unknown zone reset")

var zoneReset = enum[ZoneReset]{
	"zone reset",
	ErrUnknownZoneReset,
	[]ZoneReset{
		ZoneResetNever,
		ZoneResetNoPlayers,
		ZoneResetAlways,
	},
}

func (z *ZoneReset) UnmarshalText(b []byte) error { return zoneReset.unmarshal(z, b) }

func ParseZoneRest(s string) (ZoneReset, error) { return zoneReset.parse(s) }
