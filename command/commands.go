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

type RoomStatus struct {
	ZoneId string
	RoomId string
}

func (RoomStatus) Verb() string { return "roomstatus" }
