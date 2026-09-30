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
}
