package loader

import "io/fs"

// RoomRef names a room by the zone it is in and its id within that zone.
type RoomRef struct {
	ZoneId string `json:"zone_id"`
	RoomId string `json:"room_id"`
}

type Settings struct {
	Void     RoomRef `json:"void"`
	Start    RoomRef `json:"start"`
	Donation RoomRef `json:"donation"`
	// PlayerDeath is where a player who dies wakes up.
	PlayerDeath RoomRef `json:"player-death"`
	// Welcome is said once to a new character, after they are shown the start
	// room: where to go first. Content, not code, because it names places.
	// Empty says nothing.
	Welcome string `json:"welcome"`
}

func LoadSettings(worldFS fs.FS) (*Settings, error) {
	settings := Settings{}
	if err := loadInto(&settings, worldFS, "settings.json"); err != nil {
		return nil, err
	}
	return &settings, nil
}
