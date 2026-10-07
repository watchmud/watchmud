package loader

import (
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"time"

	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/script"
	"github.com/watchmud/watchmud/spaces"
)

// Content is the static game content read from the content directory and files: everything
// defined on disk and read-only once the server is running.
type Content struct {
	Zones    map[string]*spaces.Zone
	Settings *Settings
	Catalog  *rules.Catalog
	// Scripts is every script some mob names, compiled, by "zone/name".
	Scripts map[string]*script.Program

	// pendingDoors are doors read with the rooms, waiting for the objects
	// their keys name (doors.go).
	pendingDoors []pendingDoor
	// pendingChests likewise, for containers whose keys are objects.
	pendingChests []pendingChest
}

func NewContent(settings *Settings, catalog *rules.Catalog, zones []*spaces.Zone) *Content {
	c := Content{
		Zones:    make(map[string]*spaces.Zone),
		Settings: settings,
		Catalog:  catalog,
		Scripts:  make(map[string]*script.Program),
	}

	for _, zone := range zones {
		c.addZone(zone)
	}

	return &c
}

func LoadContent(fsys fs.FS) (*Content, error) {
	worldFS, err := fs.Sub(fsys, "world")
	if err != nil {
		return nil, fmt.Errorf("Sub(world): %w", err)
	}

	rulesFS, err := fs.Sub(fsys, "rules")
	if err != nil {
		return nil, fmt.Errorf("Sub(rules): %w", err)
	}
	settings, err := LoadSettings(worldFS)
	if err != nil {
		return nil, err
	}

	cat, err := LoadCatalog(rulesFS)
	if err != nil {
		return nil, err
	}

	// pass in empty zone list -- we'll load the zones from the manifest
	c := NewContent(settings, cat, []*spaces.Zone{})

	if err := c.loadZoneManifest(worldFS); err != nil {
		return nil, err
	}
	if err := c.loadRooms(worldFS); err != nil {
		return nil, err
	}
	// every room on its zone's grid, for a client that maps; the exits are
	// all connected by now
	for _, z := range c.Zones {
		z.LayGrid()
	}
	if err := c.loadObjectDefinitions(worldFS); err != nil {
		return nil, err
	}
	// after objects: a door's key is one, and so is a chest's
	if err := c.hangDoors(); err != nil {
		return nil, err
	}
	if err := c.fitChests(); err != nil {
		return nil, err
	}
	if err := c.loadMobileDefinitions(worldFS); err != nil {
		return nil, err
	}
	if err := c.loadShops(worldFS); err != nil {
		return nil, err
	}
	if err := c.loadZoneInstructions(worldFS); err != nil {
		return nil, err
	}
	// last: the kit names object definitions, so the zones have to be loaded
	// before anything can say whether it is valid.
	if err := c.checkStartingGear(); err != nil {
		return nil, err
	}
	return c, nil
}

func loadInto[T any](dst *T, fsys fs.FS, name string) error {
	v, err := readJSONFile[T](fsys, name)
	if err != nil {
		return err
	}
	*dst = v
	return nil
}

func (c *Content) Room(zoneId, roomId string) (*spaces.Room, error) {
	z, ok := c.Zones[zoneId]
	if !ok {
		return nil, fmt.Errorf("zone %q not found", zoneId)
	}
	r, ok := z.Rooms[roomId]
	if !ok {
		return nil, fmt.Errorf("room %q not found in zone %q", roomId, zoneId)
	}
	return r, nil
}

// Retrieve the zone manifest; prepare the zone objects to be populated by
// rooms, objects, mobiles (but don't process the zone commands yet).
func (c *Content) loadZoneManifest(fsys fs.FS) error {
	manifests, err := readJSONFile[[]zoneManifestEntry](fsys, "zone_manifest.json")
	if err != nil {
		return err
	}
	for _, m := range manifests {
		if !m.Enabled {
			continue
		}
		band, err := zonePowerBand(m)
		if err != nil {
			return err
		}
		zone := spaces.NewZone(
			m.Id,
			m.Name,
			m.ResetMode,
			time.Duration(m.LifetimeMinutes)*time.Minute,
		)
		zone.Power = band
		c.addZone(zone)
	}
	return nil
}

// Read all the room files from all the zones
func (c *Content) loadRooms(fsys fs.FS) error {

	// every room object has to exist before any exit can be connected, so
	// the parsed entries are held until the second pass.
	roomsByZone := make(map[string][]roomFileEntry, len(c.Zones))
	for _, zonename := range c.zoneNames() {
		entries, err := readJSONFile[[]roomFileEntry](fsys, path.Join(zonename, "rooms.json"))
		if err != nil {
			return err
		}
		zone := c.Zones[zonename]

		for i := range entries {
			entry := &entries[i]
			// a second room with an id would take the first one's place, and
			// its exits: the original cut off, settings pointing at the copy
			if _, dup := zone.Rooms[entry.Id]; dup {
				return fmt.Errorf("zone %s: two rooms with the id %q", zonename, entry.Id)
			}

			r := spaces.NewRoom(zone, entry.Id, entry.Name, entry.Description)
			r.SetFlags(entry.Flags)
			zone.AddRoom(r)

			// an exit that doesn't name a zone is assumed to stay in this one.
			// Index rather than range-by-value so the write actually sticks.
			for j := range entry.Exits {
				if entry.Exits[j].DestinationZoneId == "" {
					entry.Exits[j].DestinationZoneId = zonename
				}
			}
		}
		roomsByZone[zonename] = entries
	}

	// All rooms exist now, so exits can be resolved
	for _, zonename := range slices.Sorted(maps.Keys(roomsByZone)) {
		for _, entry := range roomsByZone[zonename] {
			for _, exit := range entry.Exits {
				if err := c.connectRooms(
					zonename,
					entry.Id,
					exit.Direction,
					exit.DestinationZoneId,
					exit.DestinationRoomId,
				); err != nil {
					return err
				}
				if exit.Door != nil {
					c.pendingDoors = append(c.pendingDoors, pendingDoor{zonename, entry.Id, exit.Direction, *exit.Door})
				}
			}
		}
	}
	return nil
}

func (c *Content) connectRooms(sourceZoneId string, sourceRoomId string, dir rules.Direction, destZoneId string, destRoomId string) error {
	sourceZone := c.Zones[sourceZoneId]
	if sourceZone == nil {
		return fmt.Errorf("connect rooms: source zone %q not found", sourceZoneId)
	}
	destZone := c.Zones[destZoneId]
	if destZone == nil {
		return fmt.Errorf("connect rooms: destination zone %q not found (from %s/%s going %s)",
			destZoneId, sourceZoneId, sourceRoomId, dir)
	}
	sourceRoom := sourceZone.Rooms[sourceRoomId]
	if sourceRoom == nil {
		return fmt.Errorf("connect rooms: source room %q not found in zone %q", sourceRoomId, sourceZoneId)
	}
	destRoom := destZone.Rooms[destRoomId]
	if destRoom == nil {
		return fmt.Errorf("connect rooms: destination room %q not found in zone %q (from %s/%s going %s)",
			destRoomId, destZoneId, sourceZoneId, sourceRoomId, dir)
	}
	sourceRoom.Connect(dir, destRoom)
	return nil
}

func (c *Content) addZone(z *spaces.Zone) {
	c.Zones[z.Id] = z
	// TODO should this go away?
}

// zoneNames returns the loaded zone names in a stable order, so that load
// order and error messages are reproducible run to run.
func (c *Content) zoneNames() (result []string) {
	return slices.Sorted(maps.Keys(c.Zones))
}

func (c *Content) loadObjectDefinitions(fsys fs.FS) error {
	// for each zone: create all object definitions
	for _, zonename := range c.zoneNames() {
		objEntries, err := readOptionalJSONFile[[]objectEntry](fsys, path.Join(zonename, "objects.json"))
		if err != nil {
			return err
		}
		for _, obj := range objEntries {
			// as for mobs: a second "rope" that's a lantern would make every
			// rope a lantern
			if _, dup := c.Zones[zonename].ObjectDefinitions[obj.Id]; dup {
				return fmt.Errorf("zone %s: two objects with the id %q", zonename, obj.Id)
			}
			d := object.NewDefinition(
				obj.Id,
				obj.Name,
				zonename,
				obj.Category,
				obj.Aliases,
				obj.ShortDescription,
				obj.DescriptionOnGround,
				obj.EquipmentSlot,
				obj.ArmorType,
				obj.Behaviors,
			)

			// A role id not in the catalog is a typo in content, and
			// silently ignoring it would leave a builder wondering why their
			// tank gear doesn't make anyone a tank.
			for roleId := range obj.Roles {
				if _, known := c.Catalog.Roles[roleId]; !known {
					return fmt.Errorf("object %s/%s: unknown role %q", zonename, obj.Id, roleId)
				}
			}
			d.RoleWeights = obj.Roles
			if d.Abilities, err = objectAbilities(zonename, obj, c.Catalog); err != nil {
				return err
			}
			if d.Quaff, err = objectQuaff(zonename, obj, c.Catalog); err != nil {
				return err
			}
			d.MaxDurability, err = objectDurability(zonename, obj, c.Catalog.Durability)
			if err != nil {
				return err
			}

			d.Damage, err = objectDamage(zonename, obj)
			if err != nil {
				return err
			}

			if obj.Container != nil {
				// a chest is furniture: it stays where it is (chests.go);
				// a bag is carried
				if !obj.Container.Portable && !d.NoTake() {
					d.Behaviors = append(d.Behaviors, rules.ObjectBehaviorNoTake)
				}
				c.pendingChests = append(c.pendingChests, pendingChest{zonename, d, *obj.Container})
			}

			c.Zones[zonename].AddObjectDefinition(d)
		}
	}
	return nil
}

// checkPath refuses a followPath wanderer whose path can't be walked: fewer
// than two rooms is nowhere to go (and a one-room path once indexed before
// its start), and every step names a room that exists.
func (c *Content) checkPath(zonename string, mob mobEntry) error {
	if mob.WanderingDefinition.WanderStyle != rules.WanderFollowPath {
		return nil
	}
	path := mob.WanderingDefinition.Path
	if len(path) < 2 {
		return fmt.Errorf("mob %s/%s: a followPath needs at least two rooms, has %d", zonename, mob.Id, len(path))
	}
	for _, id := range path {
		found := false
		for _, z := range c.Zones {
			if _, ok := z.Rooms[id]; ok {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("mob %s/%s: path names a room nothing has: %q", zonename, mob.Id, id)
		}
	}
	return nil
}

func (c *Content) loadMobileDefinitions(fsys fs.FS) error {
	// Summons name mobs, which may be in a zone not loaded yet: held for a
	// second pass, like room exits.
	type summoner struct {
		zone  string
		entry mobEntry
		defn  *mobile.Definition
	}
	var summoners []summoner

	for _, zonename := range c.zoneNames() {
		mobEntries, err := readOptionalJSONFile[[]mobEntry](fsys, path.Join(zonename, "mobs.json"))
		if err != nil {
			return err
		}
		for _, mob := range mobEntries {
			// a copy-pasted entry with its id unchanged would quietly
			// replace the first: every instruction naming it spawns the copy
			if _, dup := c.Zones[zonename].MobileDefinitions[mob.Id]; dup {
				return fmt.Errorf("zone %s: two mobs with the id %q", zonename, mob.Id)
			}
			if err := c.checkPath(zonename, mob); err != nil {
				return err
			}
			ac, err := mobArmorClass(zonename, mob)
			if err != nil {
				return err
			}
			damage, err := mobDamage(zonename, mob)
			if err != nil {
				return err
			}
			power, err := mobPower(zonename, mob, c.Zones[zonename].Power)
			if err != nil {
				return err
			}
			loot, err := c.mobLoot(zonename, mob)
			if err != nil {
				return err
			}
			scriptRef, err := c.mobScript(fsys, zonename, mob)
			if err != nil {
				return err
			}
			defn := mobile.NewDefinition(
				mob.Id,
				mob.Name,
				zonename,
				mob.Aliases,
				mob.ShortDescription,
				mob.DescriptionInRoom,
				mob.MaxHealth,
				rules.WanderDefinition{
					CanWander:       mob.WanderingDefinition.CanWander,
					CheckFrequency:  time.Second * time.Duration(mob.WanderingDefinition.CheckFrequencySeconds),
					CheckPercentage: float32(mob.WanderingDefinition.CheckPercentage) / 100.0,
					Style:           mob.WanderingDefinition.WanderStyle,
					Path:            mob.WanderingDefinition.Path,
				},
				ac,
				mob.Aggressive,
			)

			defn.SetFlags(mob.Flags)
			defn.Damage = damage
			defn.Power = power
			defn.Loot = loot
			defn.Script = scriptRef
			c.Zones[zonename].AddMobileDefinition(defn)
			if len(mob.Summons) > 0 {
				summoners = append(summoners, summoner{zonename, mob, defn})
			}
		}
	}
	for _, s := range summoners {
		summons, err := c.mobSummons(s.zone, s.entry)
		if err != nil {
			return err
		}
		s.defn.Summons = summons
	}
	return nil
}

func (c *Content) loadZoneInstructions(fsys fs.FS) error {
	for _, zonename := range c.zoneNames() {
		// instruction files are optional
		insts, err := readOptionalJSONFile[[]instructionFileEntry](fsys, path.Join(zonename, "instructions.json"))
		if err != nil {
			return err
		}
		zone := c.Zones[zonename]
		for _, entry := range insts {
			// zero would mean "add one every reset" to an object and "never"
			// to a mob; neither is what a missing number means
			if entry.InstanceMax < 1 {
				return fmt.Errorf("zone %s: %s %s%s in %s: instance_max must be at least 1",
					zonename, entry.Type, entry.ObjectId, entry.MobileId, entry.RoomId)
			}
			switch entry.Type {
			case "CreateObject":
				zone.AddCommand(spaces.CreateObject{
					ObjectDefinitionId: entry.ObjectId,
					RoomId:             entry.RoomId,
					ZoneId:             entry.ZoneId,
					InstanceMax:        entry.InstanceMax,
					ContainerId:        entry.Container,
					Power:              entry.Power,
				})
			case "CreateMobile":
				zone.AddCommand(spaces.CreateMobile{
					MobileDefinitionId: entry.MobileId,
					ZoneId:             entry.ZoneId,
					RoomId:             entry.RoomId,
					InstanceMax:        entry.InstanceMax,
				})
			default:
				return fmt.Errorf("zone %s: Unhandled Instruction type: %q", zonename, entry.Type)
			}
		}
	}
	return nil
}

// mobArmorClass is what the mob file said, or the unarmored baseline if it
// said nothing.
//
// Mob armor class is absolute, not a bonus: it is the number a d20 is
// compared against, exactly as a player's is, so "ac": 10 is an unarmored
// creature and "ac": 14 is one in the equivalent of plate. Defaulting an
// absent key to zero -- which is what an int field did -- made every mob
// whose file forgot it impossible to miss.
func mobArmorClass(zoneName string, mob mobEntry) (int, error) {
	if mob.AC == nil {
		return rules.BaseArmorClass, nil
	}
	if *mob.AC < 0 {
		return 0, fmt.Errorf("mob %s/%s: negative ac %d", zoneName, mob.Id, *mob.AC)
	}
	return *mob.AC, nil
}

// objectQuaff checks a potion's ability: one the catalog has, and one a
// drinker can aim at themselves -- a potion of smite has nobody to hit.
func objectQuaff(zoneName string, obj objectEntry, cat *rules.Catalog) (string, error) {
	if obj.Quaff == "" {
		return "", nil
	}
	a, known := cat.Abilities[obj.Quaff]
	if !known {
		return "", fmt.Errorf("object %s/%s: quaff: unknown ability %q", zoneName, obj.Id, obj.Quaff)
	}
	if a.Target != rules.TargetSelf && a.Target != rules.TargetFriend {
		return "", fmt.Errorf("object %s/%s: quaff: %q isn't something to drink -- its target is %q", zoneName, obj.Id, obj.Quaff, a.Target)
	}
	return obj.Quaff, nil
}

// objectAbilities checks what an object grants against the catalog: an
// unknown id is a typo, and ignoring it would leave a builder wondering why
// their censer doesn't heal.
func objectAbilities(zoneName string, obj objectEntry, cat *rules.Catalog) ([]string, error) {
	for _, id := range obj.Abilities {
		if _, known := cat.Abilities[id]; !known {
			return nil, fmt.Errorf("object %s/%s: unknown ability %q", zoneName, obj.Id, id)
		}
	}
	return obj.Abilities, nil
}
