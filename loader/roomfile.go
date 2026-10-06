package loader

import (
	"github.com/watchmud/watchmud/rules"
)

type roomFileEntry struct {
	Id          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Flags       []rules.RoomFlag `json:"flags"`
	Exits       []exit           `json:"exits"`
}

type exit struct {
	Direction         rules.Direction `json:"direction"`
	DestinationZoneId string          `json:"dest_zone"`
	DestinationRoomId string          `json:"dest_room"`
	Door              *doorEntry      `json:"door"`
}

// doorEntry is a door in an exit: declared on one side, found on both (doors.go).
type doorEntry struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
	Closed  bool     `json:"closed"`
	Locked  bool     `json:"locked"`
	// Key is the object that locks and unlocks it: a bare id is the declaring
	// zone's, "zone/id" any other. Empty is a door with no lock.
	Key string `json:"key"`
}
