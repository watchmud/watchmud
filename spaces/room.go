package spaces

import (
	"fmt"
	"math/rand"
	"slices"
	"sort"
	"strings"
	"time"
	"uuid"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/ordered"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

type Room struct {
	Id          string
	Name        string
	Description string
	Zone        *Zone
	playerList  *player.List
	Inventory   *object.List
	mobs        *ordered.List[uuid.UUID, *mobile.Instance]
	directions  map[rules.Direction]*Room
	doors       map[rules.Direction]*Door
	flags       map[rules.RoomFlag]bool
}

// NewRoom creates a new room in this zone.
// It does NOT alter the Zone to include the new room, however.
func NewRoom(zone *Zone, id string, name string, description string) *Room {
	return &Room{
		Id:          id,
		Name:        name,
		Description: description,
		Zone:        zone,
		playerList:  player.NewList(),
		Inventory:   object.NewFloor(),
		mobs:        ordered.NewList[uuid.UUID, *mobile.Instance]((*mobile.Instance).Id),
		directions:  make(map[rules.Direction]*Room),
		doors:       make(map[rules.Direction]*Door),
		flags:       make(map[rules.RoomFlag]bool),
	}
}

// NewTestRoom creates a simpler room for tests
func NewTestRoom(name string) *Room {
	return NewRoom(nil, name, name, "")
}

func (r *Room) String() string {
	return fmt.Sprintf("(Room %s-%s: '%s')", r.Zone.Id, r.Id, r.Name)
}

// ref is "zone/room", the room's name across the whole world. A test room
// has no zone, and is just its id.
func (r *Room) ref() string {
	if r.Zone == nil {
		return r.Id
	}
	return r.Zone.Id + "/" + r.Id
}

func (r *Room) Location() player.Location {
	return player.NewLocation(r.Zone.Id, r.Id)
}

func (r *Room) SetFlags(flags []rules.RoomFlag) {
	if flags != nil {
		for _, s := range flags {
			r.SetFlag(s)
		}
	}
}

func (r *Room) SetFlag(flag rules.RoomFlag) {
	r.flags[flag] = true
}

func (r *Room) Flag(flag rules.RoomFlag) bool {
	return r.flags[flag]
}

func (r *Room) Flags() []rules.RoomFlag {
	result := make([]rules.RoomFlag, 0, len(r.flags))
	for k, v := range r.flags {
		if v {
			result = append(result, k)
		}
	}
	slices.Sort(result)
	return result
}

// playerLeaves a room. Tells other room residents about it.
func (r *Room) playerLeaves(p *player.Player, dir rules.Direction) {
	r.playerList.Remove(p)
	r.Send(event.Left{Who: p.Name(), Direction: dir})
}

func (r *Room) mobileLeaves(mob *mobile.Instance, dir rules.Direction) {
	if err := r.mobs.Remove(mob); err != nil {
		log.Error().Err(err).Str("room", r.Location().String()).Msg("mobileLeaves: failed to leave room")
		return
	}
	r.Send(event.Left{Who: mob.Name(), Direction: dir})
}

// addPlayer to the Room, without sending notifications
func (r *Room) addPlayer(p *player.Player) {
	r.playerList.Add(p)
}

// removePlayer from the Room, without sending notifications
func (r *Room) removePlayer(p *player.Player) {
	r.playerList.Remove(p)
}

func (r *Room) Players() []*player.Player {
	return r.playerList.Slice()
}

// playerEnters a room, telling other room entities about it.
// Different from addPlayer, which just updates the list and
// does not send notifications.
func (r *Room) playerEnters(p *player.Player) {
	r.Send(event.Entered{Who: p.Name()})
	r.addPlayer(p)
}

// mobileEnters a room, telling other room entities about it.
func (r *Room) mobileEnters(mob *mobile.Instance) {
	if err := r.addMobile(mob); err != nil {
		log.Error().Err(err).Str("room", r.Location().String()).Msg("MobileEnters: failed to add mobile")
		return
	}
	r.Send(event.Entered{Who: mob.Definition.Name})
}

func (r *Room) addMobile(inst *mobile.Instance) error {
	return r.mobs.Add(inst)
}

func (r *Room) removeMobile(inst *mobile.Instance) error {
	return r.mobs.Remove(inst)
}

func (r *Room) Mobiles() []*mobile.Instance {
	var result []*mobile.Instance
	for m := range r.mobs.All() {
		result = append(result, m)
	}
	return result
}

// Send to every player in the room.
func (r *Room) Send(msg any) {
	for p := range r.playerList.All() {
		p.Send(msg)
	}
}

// SendExcept to one player
func (r *Room) SendExcept(exception *player.Player, msg any) {
	for p := range r.playerList.AllExcept(exception) {
		p.Send(msg)
	}
}

// Notify mobs and players in a room about something
func (r *Room) Notify(msg any) {
	// mobs
	for m := range r.mobs.All() {
		m.Send(msg)
	}
	// players
	r.Send(msg)
}

// DescriptionExcept describes the room, except for one player, if provided
// The exits are stringified for telnet clients, not the abbreviations.
func (r *Room) DescriptionExcept(exclude *player.Player) event.RoomDescription {
	desc := event.RoomDescription{
		Name:        r.Name,
		Description: r.Description,
		Exits:       r.ExitString(),
		Id:          r.ref(),
	}
	if r.Zone != nil {
		desc.Area = r.Zone.Name
	}
	for _, ex := range r.Exits(false) {
		desc.ExitTo = append(desc.ExitTo, event.ExitTo{Direction: ex.Direction, To: ex.Room.ref()})
	}

	for p := range r.playerList.AllExcept(exclude) {
		desc.Players = append(desc.Players, p.Name())
		desc.PlayerPositions = append(desc.PlayerPositions, p.Position().String())
	}

	for o := range r.Inventory.All() {
		desc.Objects = append(desc.Objects, o.Definition.DescriptionOnGround)
	}
	for mob := range r.mobs.All() {
		desc.Mobs = append(desc.Mobs, mob.Definition.DescriptionInRoom)
	}
	return desc
}

func (r *Room) FindMobile(target string) (mob *mobile.Instance, exists bool) {
	return ordered.Find(r.mobs, target)
}

func (r *Room) FindPlayer(target string) (*player.Player, bool) {
	p := r.playerList.FindByName(target)
	if p != nil {
		return p, true
	}
	return nil, false
}

// ExitString returns all the valid exits from this room as a string.
func (r *Room) ExitString() string {
	exits := r.Exits(false)
	if len(exits) == 0 {
		return rules.FormatDirection(nil)
	}
	var names []string
	for _, exit := range exits {
		name := exit.Direction.String()
		if !exit.Passable() {
			name += " (closed)"
		}
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// HasExit is whether there's a way out in this direction at all, door or no
// door. Whether it can be walked through now is Passable.
func (r *Room) HasExit(dir rules.Direction) bool {
	_, ok := r.directions[dir]
	return ok
}

// Passable is whether something can walk out this way now: there's an exit,
// and no closed door in it.
func (r *Room) Passable(dir rules.Direction) bool {
	if !r.HasExit(dir) {
		return false
	}
	d := r.doors[dir]
	return d == nil || !d.Closed
}

// DestinationRoom returns the room in this direction or nil if there isn't one,
// whether or not a door stands in the way: Passable says that.
func (r *Room) DestinationRoom(dir rules.Direction) (dest *Room) {
	return r.directions[dir]
}

// DoorTo is the door in this direction, or nil.
func (r *Room) DoorTo(dir rules.Direction) *Door {
	return r.doors[dir]
}

// SetDoor puts a door in the exit this way. Loader use only, like Connect: the
// loader gives the same door to the exit back, so both sides share it.
func (r *Room) SetDoor(dir rules.Direction, d *Door) {
	r.doors[dir] = d
}

// FindDoor is the door a player means: by direction ("west", "w"), by name or
// alias ("grate"), or by "door" when the room has just one.
func (r *Room) FindDoor(target string) (rules.Direction, *Door, bool) {
	if dir, err := rules.ParseDirectionPrefix(target); err == nil {
		if d := r.doors[dir]; d != nil {
			return dir, d, true
		}
		return rules.DirectionNone, nil, false
	}
	for _, exit := range r.Exits(false) {
		if exit.Door != nil && exit.Door.Matches(target) {
			return exit.Direction, exit.Door, true
		}
	}
	if strings.EqualFold(strings.TrimSpace(target), "door") && len(r.doors) == 1 {
		for dir, d := range r.doors {
			return dir, d, true
		}
	}
	return rules.DirectionNone, nil, false
}

// Connect this room to the destination room in this direction.
// Loader use only: room topology is immutable once content is loaded.
// See ROADMAP "Known Problems": room conflates definition and instance.
func (r *Room) Connect(dir rules.Direction, destRoom *Room) {
	r.directions[dir] = destRoom
}

// PickRandomDirection from here to travel, from directions that are available.
// If there aren't any, return direction.None.
func (r *Room) PickRandomDirection(limitToZone bool) rules.Direction {
	// TODO should this return a room and not a direction?
	var exits []RoomExit
	for _, re := range r.Exits(limitToZone) {
		if re.Passable() {
			exits = append(exits, re)
		}
	}
	if len(exits) == 0 {
		return rules.DirectionNone
	} else {
		desired := rand.New(rand.NewSource(time.Now().Unix())).Int31n(int32(len(exits)))
		// iterate to the ith member of exits
		i := int32(0)
		for _, re := range exits {
			if i == desired {
				return re.Direction
			}
			i++
		}
		// inconceivable!
		log.Warn().Msgf("Room.PickRandomDirection: Bizarre RandomDirection picked. len=%d, desired=%d", len(exits), desired)
		return rules.DirectionNone
	}
}

// Exits returns the exits from this room.
// Uses the direction.Direction ordering.
// Every exit, a closed door's included: RoomExit.Passable says which can be
// walked through.
func (r *Room) Exits(limitToZone bool) []RoomExit {
	holder := roomExitHolder{}
	for dir, dest := range r.directions {
		if !limitToZone || r.Zone == dest.Zone {
			holder.dirs = append(holder.dirs, RoomExit{dir, dest, r.doors[dir]})
		}
	}
	sort.Sort(holder)
	return holder.dirs
}
