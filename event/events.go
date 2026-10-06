package event

import (
	"time"

	"github.com/watchmud/watchmud/rules"
)

// ---- session ---------------------------------------------------------------
//
// These four are read by the connection's login conversation
// (telnet/conn.go), not by the renderer. They are four types rather than two
// with a Success field because a type switch there stays honest in a way a
// bool doesn't.

type LoggedIn struct {
	Name string
}

type LoginFailed struct {
	Reason ResultCode
}

type PlayerCreated struct {
	Name string
}

type CreateFailed struct {
	Reason ResultCode
}

// LoggedOut reaches the room the player just left, so in practice the player
// themselves never sees it -- they are removed from the room first, and their
// own goodbye comes from the connection.
type LoggedOut struct {
	Actor string
}

// Prompt says the game is waiting on the player's input, and carries what
// the prompt shows. The server sends it to everyone after every command and
// every pulse; it is the transport's job to print it only when something was
// said since the last one.
type Prompt struct {
	CurrentHealth int
	MaxHealth     int
	CurrentMana   int
	MaxMana       int // zero: no mana pool, and the prompt doesn't mention one
}

// Welcome is said once to a brand-new character, after they've been shown the
// start room. The text is content (settings.json "welcome").
type Welcome struct {
	Text string
}

// EnteredGame tells a room that a player just logged in there. Not LoggedIn,
// which the arriving player's own connection consumes to end its login
// conversation -- a bystander's connection would do the same with it.
// EnteredGame is someone appearing in the room by logging in. First is a
// character's very first time, straight from creation.
type EnteredGame struct {
	Actor string
	First bool
}

type Pong struct {
	Target string
}

// ---- rooms -----------------------------------------------------------------

// RoomDescription is what a player sees of a room. One event covers look,
// move and recall, which have always rendered identically.
type RoomDescription struct {
	Name        string
	Description string
	Exits       string
	Players     []string
	Objects     []string
	Mobs        []string

	// Where the room is, for a client that maps (GMCP Room.Info): "zone/room",
	// the zone's name, and where each exit leads. Nothing renders them.
	Id     string
	Area   string
	ExitTo []ExitTo
}

// ExitTo is one way out of a room and the room it leads to, "zone/room".
type ExitTo struct {
	Direction rules.Direction
	To        string
}

type Entered struct {
	Who string
}

type Left struct {
	Who       string
	Direction rules.Direction
}

type Exits struct {
	Exits []Exit
}

type Exit struct {
	Direction rules.Direction
	RoomName  string
	Closed    bool // a closed door stands in it
}

// DoorChange is what happened to a door.
type DoorChange string

const (
	DoorOpened   DoorChange = "opened"
	DoorClosed   DoorChange = "closed"
	DoorLocked   DoorChange = "locked"
	DoorUnlocked DoorChange = "unlocked"
)

// ContainerChanged is a chest opened, closed, locked or unlocked, seen by
// the room it's in.
type ContainerChanged struct {
	Actor     string
	Container string
	Change    DoorChange
}

// DoorChanged goes to both rooms a door joins. Direction is the door's from
// the room hearing it; Actor is empty on the far side, where nobody saw who.
type DoorChanged struct {
	Actor     string
	Door      string
	Direction rules.Direction
	Change    DoorChange
}

// ---- objects ---------------------------------------------------------------
//
// Dropped and Got are each one event for both audiences: the player who acted
// and everyone else in the room. The renderer compares Actor to the name of
// the player it is rendering for, the same way it already does for Struck.

type Dropped struct {
	Actor string
	Item  string
}

// Put is something put into a container, seen by the room.
type Put struct {
	Actor string
	Item  string
	Into  string
}

// Following is a follow starting or stopping, told to both ends of it.
type Following struct {
	Follower string
	Leader   string
	Stopped  bool
}

// Followed is a follower walking after their leader, or -- Fighting -- being
// left behind by a fight.
type Followed struct {
	Leader    string
	Direction rules.Direction
	Fighting  bool
}

// GroupList is "group": the leader first, then followers in the order they
// joined.
type GroupList struct {
	Members []GroupMember
}

type GroupMember struct {
	Name      string
	Leader    bool
	Health    int
	MaxHealth int
	Mana      int
	MaxMana   int
	Room      string
}

// Assisted is a group member joining another's fight, seen by the room.
type Assisted struct {
	Actor  string
	Member string
	Target string
}

// AssistSet answers assist: whether you'll now join your group's fights.
type AssistSet struct {
	On bool
}

// GroupTold is a gtell, to everyone in the group, the speaker included.
type GroupTold struct {
	Speaker string
	Value   string
}

// Swept is a sweeper -- the janitor -- taking junk off the floor for good.
type Swept struct {
	Sweeper string
	Item    string
}

// Gave is one thing, or some coins, passed from Actor to Recipient: the
// giver, the one given to and the room each see it their own way.
type Gave struct {
	Actor     string
	Recipient string
	Item      string
}

type Got struct {
	Actor string
	Item  string
	// From is the container it came out of, empty for the floor.
	From string
}

// ContainerContents answers "look in": what a container holds, in the order
// it was put there.
type ContainerContents struct {
	Container string
	Items     []ContainedItem
	Coins     int
}

// Crumbled goes to the room: a summon going back to where it came from, at the
// end of its summoner's fight or its own. It leaves nothing behind.
type Crumbled struct {
	Name string
}

// Decayed is something on the floor crumbling away: a corpse, so far.
type Decayed struct {
	Item string
}

type ContainedItem struct {
	ShortDescription string
	Power            int
}

// Equipped and Worn have no bystander text today, so they carry no Actor.
// Equipped names what went into the slot -- the hand, for wield.
type Equipped struct {
	Item string
}

// Worn names what went on, since "wear leather" might mean a cap or boots.
type Worn struct {
	Item string
}

// GoldGiven is a wizard's gold: how many coins appeared, and the purse after.
type GoldGiven struct {
	Amount int
	Coins  int
}

// Removed names the item so "Removed." doesn't leave the player guessing
// which of two similarly named things came off.
type Removed struct {
	Item string
}

// Repaired is one piece of gear made as good as new, and what it cost. Only
// its owner hears it.
type Repaired struct {
	Item string
	Cost int
}

// ShopList is what a shop sells, and for how much.
type ShopList struct {
	Items []ShopEntry
}

type ShopEntry struct {
	Item  string
	Power int
	Price int
}

// Bought, Sold and Valued go to the trader alone.
type Bought struct {
	Item string
	Cost int
}

type Sold struct {
	Item  string
	Coins int
}

type Valued struct {
	Item  string
	Coins int
}

// TooExpensive is something the player wanted and couldn't pay for: what it
// would have cost, and what they have. A failure with numbers in it, which
// Failed can't carry.
type TooExpensive struct {
	Item  string
	Cost  int
	Coins int
}

type Inventory struct {
	Items []InventoryItem
	Coins int
}

type InventoryItem struct {
	Id               string
	ShortDescription string
	Category         rules.ObjectCategory
	// Power and condition, as EquippedItem has them: what a player weighs
	// before wearing or selling something.
	Power         int
	Durability    int
	MaxDurability int
	Broken        bool
	// Bag is whether it holds things, and Holding how many it does.
	Bag     bool
	Holding int
}

type Equipment struct {
	// Power is the player's, the average of Items that aren't broken.
	Power int
	Items []EquippedItem
}

type EquippedItem struct {
	Slot             rules.EquipmentSlot
	Id               string
	ShortDescription string

	// Durability and MaxDurability are what the piece has left and what it
	// started with. Both zero means gear that doesn't wear out, which the
	// renderer says nothing about; Broken is called out on its own because it
	// is the state that changes what the piece is doing for you.
	Durability    int
	MaxDurability int
	Broken        bool
	// Power is this particular piece's.
	Power int
}

// ---- talking ---------------------------------------------------------------

type Said struct {
	Speaker string
	Value   string
}

// Told goes to both parties: the renderer shows the sender an acknowledgement
// and the receiver the message.
type Told struct {
	From  string
	To    string
	Value string
}

type Shouted struct {
	Speaker string
	Value   string
}

// ---- the player -----------------------------------------------------------

// Color is whether a player's connection shows ANSI color. Changed is the
// player having just asked; without it this is the setting being applied at
// login, which says nothing.
type Color struct {
	On      bool
	Changed bool
}

type Who struct {
	Players []WhoEntry
}

type WhoEntry struct {
	PlayerName string
	// Bot is the record's flag: a program plays this character.
	Bot     bool
	Lineage string
	// Role is what that player's equipment adds up to right now, and is
	// empty when it adds up to nothing.
	Role     string
	ZoneName string
	RoomName string
}

type Stat struct {
	PlayerName string
	Lineage    string
	// Role, empty when the player is wearing nothing that speaks to one.
	Role          string
	Power         int
	ArmorClass    int
	CurrentHealth int
	MaxHealth     int
	CurrentMana   int
	MaxMana       int
	Coins         int
	// where they are, by name
	RoomName string
	ZoneName string
}

// Considered answers consider. Delta is yours less theirs, clamped the way
// combat clamps it (rules.PowerDelta), so the renderer's wording tracks what
// the fight would actually do rather than the raw gap.
type Considered struct {
	Target      string
	TargetPower int
	YourPower   int
	Delta       int
}

// Role is the answer to "what am I, and why": the role the player's equipment
// currently adds up to, plus the standings of every other role and the items
// arguing for them. The breakdown is the point -- without it "you are a Tank"
// is a verdict with no way to appeal, and the player can't tell what to swap.
type Role struct {
	// Current is the winning role's name, empty if no equipment speaks to any
	// role at all.
	Current     string
	Description string
	// Standings covers every role the game defines, in content order, so a
	// role with nothing behind it still shows up as somewhere to go.
	Standings []RoleStanding
}

type RoleStanding struct {
	Name  string
	Total int
	// Sources are the equipped items contributing to this role, already
	// formatted as "iron helmet 2", in slot order.
	Sources []string
}

// ---- combat ----------------------------------------------------------------

// Attacking acknowledges that a fight has started.
type Attacking struct {
	Target string
}

// Struck is one swing, seen by the whole room. The renderer picks the second
// person for whichever end of it is reading. Damage is what got through;
// Absorbed is what the target's ward took first.
type Struck struct {
	Attacker string
	Target   string
	Hit      bool
	Damage   int
	Absorbed int
}

// Broke is a piece of equipment giving out, seen by the room the way a blow
// is: the renderer tells the owner it was theirs. Broken gear stays worn and
// stays carried -- what it stops doing is counting for anything.
//
// Item is the object's Name rather than its ShortDescription, because this is
// the one message that reads possessively and "your a chain shirt" is not a
// sentence.
type Broke struct {
	Actor string
	Item  string
}

// GearDamaged is the toll dying takes on everything you were wearing. It goes
// to the player alone -- what the room sees is the dying, and anything that
// actually broke says so itself.
type GearDamaged struct {
	Items int
}

type Died struct {
	Target   string
	IsPlayer bool
}

// NoHassle answers the nohassle command: whether aggressive mobs now leave
// the wizard alone.
type NoHassle struct {
	On bool
}

// Slain is a wizard killing a mob outright; event.Died follows it, as it
// follows any killing blow.
type Slain struct {
	Actor  string
	Target string
}

type Restored struct {
	Target   string
	IsPlayer bool
}

// Healed goes to the whole room. Amount is what was actually restored, which
// is 0 when the target wasn't hurt: the cast still happened.
type Healed struct {
	Actor  string
	Target string
	Amount int
}

// Smote goes to the whole room: a smite always lands, so there's no Hit.
type Smote struct {
	Actor  string
	Target string
	Damage int
}

// Provoked goes to the whole room: Target has turned on Actor. Already means
// it was fighting them anyway, and the cast was wasted.
type Provoked struct {
	Actor   string
	Target  string
	Already bool
}

// Stunned goes to the whole room: Target will skip its next Rounds swings.
type Stunned struct {
	Actor  string
	Target string
	Rounds int
}

// Warded goes to the whole room: Target's ward will take the next Amount
// damage. Amount is the ward as it now stands, which after a refresh may be
// what was left of the old one.
type Warded struct {
	Actor  string
	Target string
	Amount int
}

// WardBroken is a ward used up by a blow, after the Struck that did it. One
// that runs out of time fades without a word.
type WardBroken struct {
	Name string
}

// Assessed is a mob's numbers, read by an assess: the room sees it happen,
// and the renderer gives the numbers to the one who cast it. Fighting is who
// the mob is swinging at, empty if nobody; Stunned is rounds it has left to
// skip.
type Assessed struct {
	Actor      string
	Target     string
	Health     int
	MaxHealth  int
	ArmorClass int
	Power      int
	Damage     string
	Fighting   string
	Stunned    int
}

// LookedAtObject is "look <thing>" on an object: what one thing is, for the
// one looking. Slot is rules.SlotNone for something nobody wears; Damage is
// empty for anything not wielded; Armor is what it adds to armor class.
// Abilities are names, in the item's order.
type LookedAtObject struct {
	Item          string
	Slot          rules.EquipmentSlot
	ArmorType     rules.ArmorType
	Worn          bool
	Power         int
	Durability    int
	MaxDurability int
	Broken        bool
	Damage        string
	Armor         int
	Abilities     []string
	// a container: a bag, or a chest with a lid
	Container bool
	Holding   int
	Capacity  int
	Closed    bool
	Locked    bool
}

// LookedAtMob is "look <mob>": how it seems, not its numbers -- those are
// assess's. Health is a percentage the renderer puts into words.
type LookedAtMob struct {
	Name     string
	Health   int
	Fighting string
}

// LookedAtPlayer is "look <player>": who they are and what they have on.
type LookedAtPlayer struct {
	Name     string
	Lineage  string
	Role     string
	Health   int
	Fighting string
	Wearing  []string
}

// Staggered is a stunned combatant's round going by without a swing.
type Staggered struct {
	Name string
}

type Abilities struct {
	Granted []GrantedAbility
}

type GrantedAbility struct {
	Name     string
	Mana     int
	Cooldown time.Duration
	Item     string // short description of the item granting it
	Power    int
	ReadyIn  time.Duration // zero: ready
}
type Fleeing struct {
	Who string
}
type FleeAttemptFailed struct {
	Who string
}

type Fled struct {
	Who string
}

// Received is starting gear handed to a character from before it was in the
// kit, at their login: worn if its slot was free, otherwise in their pack.
type Received struct {
	Item string
	Worn bool
}

// Summoned goes to the room: Summoner called up a Count of Name, who arrive fighting.
type Summoned struct {
	Summoner string
	Name     string
	Count    int
}

// ---- builder commands ------------------------------------------------------

type Loaded struct{}

type RoomStatus struct {
	Id          string
	Name        string
	Description string
	ZoneId      string
	ZoneName    string
	Flags       []rules.RoomFlag
	Players     []RoomStatusPlayer
	Items       []RoomStatusItem
	Mobs        []RoomStatusMob
	Exits       []RoomStatusExit
}

type RoomStatusPlayer struct {
	Name          string
	CurrentHealth int
	MaxHealth     int
}

type RoomStatusItem struct {
	Id                  string
	DefinitionId        string
	ZoneId              string
	Name                string
	ShortDescription    string
	DescriptionOnGround string
	Aliases             []string
	Category            rules.ObjectCategory
	Behaviors           []rules.ObjectBehavior
}

type RoomStatusMob struct {
	Id                string
	DefinitionId      string
	ZoneId            string
	Name              string
	ShortDescription  string
	DescriptionInRoom string
	Aliases           []string
	Flags             []rules.MobileFlag
	CurrentHealth     int
	MaxHealth         int
}

type RoomStatusExit struct {
	Direction rules.Direction
	RoomId    string
	ZoneId    string
	Flags     []rules.RoomFlag
}
