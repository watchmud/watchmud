package spaces

import "fmt"

// ZoneCommand describes all Zone Commands
type ZoneCommand any

// CreateObject instructs the world to create an object in the world.
type CreateObject struct {
	ObjectDefinitionId string // what type of object
	ZoneId             string // which zone is the definition in, leave empty for "this zone"
	RoomId             string // where does the object go?
	// InstanceMax is how many a reset tops the room -- or the container -- up
	// to. Zero is no limit.
	InstanceMax int
	// ContainerId, if set, puts it inside the first container of that
	// definition in the room, rather than on the floor.
	ContainerId string
	// Power is what it's made at; nil is the bottom of the zone's band.
	Power *int
}

func (cmd CreateObject) String() string {
	return fmt.Sprintf("Create Object '%s-%s' in Room '%s', Max of %d",
		cmd.ObjectDefinitionId,
		cmd.ZoneId,
		cmd.RoomId,
		cmd.InstanceMax,
	)
}

// CreateMobile creates a mobile instance somewhere in the world
type CreateMobile struct {
	MobileDefinitionId string // what type of mobile
	ZoneId             string // where the mobile is defined, or empty for "this zone"
	RoomId             string // where does the mobile go?
	InstanceMax        int    // how many are allowed to be walking around the zone?
	// TODO give equipment
	// TODO give objects
}

func (cmd CreateMobile) String() string {
	return fmt.Sprintf("Create Mobile '%s-%s' in Room '%s', Max of %d",
		cmd.MobileDefinitionId,
		cmd.ZoneId,
		cmd.RoomId,
		cmd.InstanceMax,
	)
}

// TODO: other types
