package loader

// instruction files are optional for a zone
type instructionFileEntry struct {
	Type        string `json:"type"`
	ObjectId    string `json:"object_id"`
	MobileId    string `json:"mobile_id"`
	ZoneId      string `json:"zone_id"`
	RoomId      string `json:"room_id"`
	InstanceMax int    `json:"instance_max"`
	// CreateObject only: put it in this container in the room, and make it
	// at this power (otherwise the zone band's bottom).
	Container string `json:"container"`
	Power     *int   `json:"power"`
}
