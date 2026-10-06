package spaces

import (
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

type Zone struct {
	Id                string
	Rooms             map[string]*Room              // id -> *room
	ObjectDefinitions map[string]*object.Definition // id (no zone) -> object.Definition
	MobileDefinitions map[string]*mobile.Definition // id -> mobile.Definition
	Name              string
	Commands          []ZoneCommand
	ResetMode         rules.ZoneReset
	LastReset         time.Time
	Lifetime          time.Duration
	// Power is the band this zone is built for. See LEVELS.md.
	Power rules.PowerBand
	// Shops is the rooms that trade, by room id.
	Shops map[string]*Shop
	// Doors is the doors this zone's content declared, which its resets put
	// back how they started.
	Doors []*Door
}

func NewZone(id string, name string, resetMode rules.ZoneReset, lifetime time.Duration) *Zone {
	return &Zone{
		Id:                id,
		Name:              name,
		Rooms:             make(map[string]*Room),
		ObjectDefinitions: make(map[string]*object.Definition),
		MobileDefinitions: make(map[string]*mobile.Definition),
		Shops:             make(map[string]*Shop),
		ResetMode:         resetMode,
		Lifetime:          lifetime,
	}
}

func (z *Zone) AddRoom(r *Room) {
	r.Zone = z
	z.Rooms[r.Id] = r
}

func (z *Zone) AddObjectDefinition(obj *object.Definition) {
	z.ObjectDefinitions[obj.ObjectId.DefinitionId] = obj
}

func (z *Zone) AddMobileDefinition(mob *mobile.Definition) {
	mob.ZoneId = z.Id
	z.MobileDefinitions[mob.Id] = mob
}

func (z *Zone) AddCommand(c ZoneCommand) {
	z.Commands = append(z.Commands, c)
}

func (z *Zone) String() string {
	return fmt.Sprintf("(Zone %s: '%s')", z.Id, z.Name)
}

func (z *Zone) Reset(o *Occupancy) []error {
	log.Debug().Str("zone", z.Name).Msg("reset")
	errs := []error{}
	for _, cmd := range z.Commands {
		switch cmd.(type) {
		case CreateMobile:
			var err error
			if err = z.createMobile(o, cmd.(CreateMobile)); err != nil {
				errs = append(errs, err)
			}
		case CreateObject:
			var err error
			if err = z.createObject(cmd.(CreateObject)); err != nil {
				errs = append(errs, err)
			}
		default:
			errs = append(errs, fmt.Errorf("zone %s unhandled Zone Command Type: %s", z.Id, cmd))
		}

	}

	for _, d := range z.Doors {
		d.Reset()
	}

	// Set the last reset time, even if there were errors. Whatever
	// was wrong is probably still wrong the next time, no reason to
	// start spinning errors here.
	z.LastReset = time.Now()

	return errs
}

func (z *Zone) createMobile(occ *Occupancy, cmd CreateMobile) error {
	d := z.MobileDefinitions[cmd.MobileDefinitionId]
	if d == nil {
		return errors.New(fmt.Sprintf("createMobile: definition id not found: %s", cmd))
	}
	// how many of this mobile definition id are in the zone?
	if occ.MobileCount(d.Id) < cmd.InstanceMax {
		r := z.Rooms[cmd.RoomId]
		if r == nil {
			return errors.New(fmt.Sprintf("createMobile: room not found: %s", cmd))
		}
		log.Printf("Creating mobile: %s", d.Id)
		occ.PlaceMobile(mobile.NewInstance(d), r)
	}
	return nil
}

// Create an object and put it in a room. If there was an error, return the error.
// createObject tops a room's floor, or a container in it, up to InstanceMax of
// this definition. A container already there is reset, lid and lock, to how its
// definition starts it: a reset closes and locks what content said was.
func (z *Zone) createObject(cmd CreateObject) error {
	defn := z.ObjectDefinitions[cmd.ObjectDefinitionId]
	if defn == nil {
		return fmt.Errorf("createObject: definition id not found: %s", cmd)
	}
	r := z.Rooms[cmd.RoomId]
	if r == nil {
		return fmt.Errorf("createObject: room not found: %s", cmd)
	}
	into := r.Inventory
	if cmd.ContainerId != "" {
		container := firstOf(r.Inventory, cmd.ContainerId)
		if container == nil || container.Contents == nil {
			return fmt.Errorf("createObject: no container %q in the room for %s", cmd.ContainerId, cmd)
		}
		into = container.Contents
	}
	if existing := countOf(into, cmd.ObjectDefinitionId); cmd.InstanceMax > 0 && existing >= cmd.InstanceMax {
		if c := firstOf(into, cmd.ObjectDefinitionId); c != nil && c.Lock != nil {
			c.Lock.Reset()
		}
		return nil
	}
	inst := object.NewInstance(uuid.New(), defn)
	inst.Power = z.Power.Min
	if cmd.Power != nil {
		inst.Power = *cmd.Power
	}
	return into.Add(inst)
}

// firstOf is the first object of this definition in the list, or nil.
func firstOf(l *object.List, definitionId string) *object.Instance {
	for inst := range l.All() {
		if inst.Definition.ObjectId.DefinitionId == definitionId {
			return inst
		}
	}
	return nil
}

func countOf(l *object.List, definitionId string) int {
	n := 0
	for inst := range l.All() {
		if inst.Definition.ObjectId.DefinitionId == definitionId {
			n++
		}
	}
	return n
}
