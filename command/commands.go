package command

import (
	"github.com/watchmud/watchmud/rules"
)

// ---- session ---------------------------------------------------------------

type Login struct {
	Name     string
	Password Secret
}

func (Login) Verb() string { return "login" }

type CreatePlayer struct {
	Name     string
	Password Secret
	// Lineage is a rules.Lineage id, and the only choice creation makes. It
	// is cosmetic: there is no class to pick beside it, because what a
	// character is good at comes from the gear they put on.
	Lineage string
	// TODO other fields ... see server/commands.go
}

func (CreatePlayer) Verb() string { return "create" }

type Logout struct {
	Cause string
}

func (Logout) Verb() string { return "quit" }

type Ping struct {
	Target string
}

func (Ping) Verb() string { return "ping" }

// ---- looking and moving ----------------------------------------------------

type Look struct {
	// Target is what the player wants to look at. Empty means the room.
	Target string
	// In is "look in <target>": what's inside a container.
	In bool
}

func (Look) Verb() string { return "look" }

type Move struct {
	Direction rules.Direction
}

func (Move) Verb() string { return "move" }

type Exits struct{}

func (Exits) Verb() string { return "exits" }

type Recall struct{}

func (Recall) Verb() string { return "recall" }

// ---- objects ---------------------------------------------------------------
//
// Target strings are raw: world.parseTarget owns the "all", "all.knife",
// "2.knife", "20 coins" grammar. Don't parse them in a transport.

type Get struct {
	Target string
	// From is the container to get it out of: "get knife from corpse".
	// Empty means the floor.
	From string
}

func (Get) Verb() string { return "get" }

// Put is "put knife in chest": something carried into an open container on
// the floor. Target takes get's grammar, coins included.
type Put struct {
	Target string
	Into   string
}

func (Put) Verb() string { return "put" }

// Give is "give knife to bob": something carried, or coins, to another player
// in the same room. Target takes get's grammar.
type Give struct {
	Target string
	To     string
}

func (Give) Verb() string { return "give" }

// Follow is "follow ann": walk where they walk. No target, or your own name,
// stops following.
type Follow struct {
	Target string
}

func (Follow) Verb() string { return "follow" }

// Ungroup is the leader's: "ungroup bob" stops one follower, "ungroup" all.
type Ungroup struct {
	Target string
}

func (Ungroup) Verb() string { return "ungroup" }

// Assist is "assist [on|off]": whether you join your group's fights. Just
// "assist" switches it.
type Assist struct {
	Setting string
}

func (Assist) Verb() string { return "assist" }

// Group lists the group you're in.
type Group struct{}

func (Group) Verb() string { return "group" }

// GroupTell is "gtell hello": to everyone in your group, wherever they are.
type GroupTell struct {
	Value string
}

func (GroupTell) Verb() string { return "gtell" }

type Drop struct {
	Target string
}

func (Drop) Verb() string { return "drop" }

// Remove takes an equipped item back off. It is the other half of wear and
// wield: without it a character can add gear but never swap it, and a role is
// only really chosen by equipment if it can be unchosen the same way.
type Remove struct {
	Target string
}

func (Remove) Verb() string { return "remove" }

// Repair is mending worn gear, at a smithy. Target takes the usual grammar.
type Repair struct {
	Target string
}

func (Repair) Verb() string { return "repair" }

// Open, Close, Lock and Unlock act on a door: its name, an alias, or the
// direction it's in. Lock and Unlock need its key in hand.
type Open struct{ Target string }
type Close struct{ Target string }
type Lock struct{ Target string }
type Unlock struct{ Target string }

func (Open) Verb() string   { return "open" }
func (Close) Verb() string  { return "close" }
func (Lock) Verb() string   { return "lock" }
func (Unlock) Verb() string { return "unlock" }

// ---- trading -----------------------------------------------------------------

// List is what a shop has for sale.
type List struct{}

func (List) Verb() string { return "list" }

// Buy takes the target grammar, matched against a shop's stock.
type Buy struct {
	Target string
}

func (Buy) Verb() string { return "buy" }

// Sell takes the target grammar, matched against what the player carries.
type Sell struct {
	Target string
}

func (Sell) Verb() string { return "sell" }

// Value is what a shop would pay for something, without selling it.
type Value struct {
	Target string
}

func (Value) Verb() string { return "value" }

type Wear struct {
	Target string
}

func (Wear) Verb() string { return "wear" }

type Equip struct {
	Target string
	Slot   rules.EquipmentSlot
}

func (Equip) Verb() string { return "equip" }

type Inventory struct{}

func (Inventory) Verb() string { return "inventory" }

type ShowEquipment struct{}

func (ShowEquipment) Verb() string { return "equipment" }

// ---- talking ---------------------------------------------------------------

type Say struct {
	Value string
}

func (Say) Verb() string { return "say" }

type Tell struct {
	To    string
	Value string
}

func (Tell) Verb() string { return "tell" }

type TellAll struct {
	Value string
}

func (TellAll) Verb() string { return "tellall" }

// ---- the player -----------------------------------------------------------

type Who struct{}

func (Who) Verb() string { return "who" }

// Color is "color on", "color off", or a bare "color" to switch it.
type Color struct {
	Setting string
}

func (Color) Verb() string { return "color" }

type Stat struct{}

func (Stat) Verb() string { return "stat" }

// Role asks which role the player's equipment adds up to, and how close the
// others are. There is nothing to change here -- you change your role by
// changing your gear -- so it takes no arguments.
type Role struct{}

func (Role) Verb() string { return "role" }

// Cast uses an ability the player's gear grants: cast heal, cast heal bob.
// Ability is the first word as typed; Target is the rest, raw.
type Cast struct {
	Ability string
	Target  string
}

func (Cast) Verb() string { return "cast" }

// Abilities lists what the player's gear lets them cast, right now.
type Abilities struct{}

func (Abilities) Verb() string { return "abilities" }

// ---- combat ----------------------------------------------------------------

type Kill struct {
	Target string
}

func (Kill) Verb() string { return "kill" }

type Flee struct{}

func (Flee) Verb() string { return "flee" }

// Consider sizes up a mob before you start something you can't finish.
type Consider struct {
	Target string
}

func (Consider) Verb() string { return "consider" }

// ---- builder commands ------------------------------------------------------

// Wizard marks a builder command. The world refuses one from anyone whose
// record doesn't say they're a wizard, before any handler runs. The method is
// unexported so only this package can mark a command, and a builder command
// without it is open to everyone: add it to world/wizard_test.go's list too.
type Wizard interface {
	Command
	wizard()
}

func (Load) wizard()       {}
func (Restore) wizard()    {}
func (RoomStatus) wizard() {}
func (NoHassle) wizard()   {}
func (Slay) wizard()       {}
func (Gold) wizard()       {}

type Load struct {
	Type string // "mob" or "obj"
	Zone string // empty means the room's own zone
	Id   string
}

func (Load) Verb() string { return "load" }

type Restore struct {
	Target string
}

func (Restore) Verb() string { return "restore" }

// NoHassle switches aggressive mobs leaving the wizard alone: "on", "off",
// or empty to flip it.
type NoHassle struct {
	Setting string
}

func (NoHassle) Verb() string { return "nohassle" }

// Slay kills a mob in the room outright, through the ordinary death: corpse,
// loot and all. For testing; never a player.
type Slay struct {
	Target string
}

func (Slay) Verb() string { return "slay" }

// Gold puts coins in the wizard's own purse, for testing what coins buy.
type Gold struct {
	Amount string
}

func (Gold) Verb() string { return "gold" }

type RoomStatus struct {
	ZoneId string
	RoomId string
}

func (RoomStatus) Verb() string { return "roomstatus" }
