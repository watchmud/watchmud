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

// Social is a gesture from rules/socials.json: "smile", "bow bob". It is
// what the parser makes of any verb it doesn't know, so Verb is the name
// typed -- one that names no social fails as an unknown request.
type Social struct {
	Name   string
	Target string
}

func (s Social) Verb() string { return s.Name }

// Socials lists them.
type Socials struct{}

func (Socials) Verb() string { return "socials" }

// Emote is "emote waves hello" or ": waves hello": the room reads the
// player's name and then the text.
type Emote struct {
	Text string
}

func (Emote) Verb() string { return "emote" }

// Reply is a tell to whoever last told you something.
type Reply struct {
	Value string
}

func (Reply) Verb() string { return "reply" }

// Whisper is "whisper bob hello" or, with Ask, "ask bob hello": to one in the
// room, player or mob; the rest of the room sees that something was said.
type Whisper struct {
	To    string
	Value string
	Ask   bool
}

func (w Whisper) Verb() string {
	if w.Ask {
		return "ask"
	}
	return "whisper"
}

// Toggle is "toggle" for the list of what a player can switch, or "toggle
// tell" to switch one. notell and noshout are its shorthands.
type Toggle struct {
	Name string
}

func (Toggle) Verb() string { return "toggle" }

// Junk is "junk <item>": destroyed, nothing back.
type Junk struct {
	Target string
}

func (Junk) Verb() string { return "junk" }

// Donate is "donate <item>": sent to the donation room, from anywhere.
type Donate struct {
	Target string
}

func (Donate) Verb() string { return "donate" }

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

// OOC is the out-of-character channel: "ooc <words>" to everyone on it,
// "ooc on" and "ooc off" to join and leave it.
type OOC struct {
	Value string
}

func (OOC) Verb() string { return "ooc" }

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

// Talk marks a command that speaks to other players: a muted player can't
// use one. The world checks it before any handler runs, as it does Wizard.
type Talk interface {
	Command
	talk()
}

func (Say) talk()       {}
func (Tell) talk()      {}
func (TellAll) talk()   {}
func (OOC) talk()       {}
func (Emote) talk()     {}
func (Whisper) talk()   {}
func (Reply) talk()     {}
func (Social) talk()    {}
func (GroupTell) talk() {}

// Position is sit, rest, sleep, stand and wake: To says which. Wake is
// standing up from sleep.
type Position struct {
	To   string // "sit", "rest", "sleep", "stand"
	Wake bool
}

func (p Position) Verb() string {
	if p.Wake {
		return "wake"
	}
	return p.To
}

// Report is bug, idea or typo: a note for whoever runs the game.
type Report struct {
	Kind string
	Text string
}

func (r Report) Verb() string { return r.Kind }

// Reports lists the latest reports, for a wizard.
type Reports struct{}

func (Reports) Verb() string { return "reports" }
func (Reports) wizard()      {}

// Goto takes a wizard to a room ("zone/room"), a player, or a mob.
type Goto struct{ Target string }

func (Goto) Verb() string { return "goto" }

// Transfer brings a player to the wizard.
type Transfer struct{ Target string }

func (Transfer) Verb() string { return "transfer" }

// Purge clears the room of mobs and things on the floor, or one of them.
type Purge struct{ Target string }

func (Purge) Verb() string { return "purge" }

// ZReset resets a zone now: the wizard's, or one named.
type ZReset struct{ Zone string }

func (ZReset) Verb() string { return "zreset" }

// Echo puts text in front of the room, or with Global everyone playing.
type Echo struct {
	Text   string
	Global bool
}

func (e Echo) Verb() string {
	if e.Global {
		return "gecho"
	}
	return "echo"
}

// Users lists who is playing and where.
type Users struct{}

func (Users) Verb() string { return "users" }

// Moderate is mute and freeze, switched for one player.
type Moderate struct {
	Target string
	Freeze bool // else mute
}

func (m Moderate) Verb() string {
	if m.Freeze {
		return "freeze"
	}
	return "mute"
}

func (Goto) wizard()       {}
func (Transfer) wizard()   {}
func (Purge) wizard()      {}
func (ZReset) wizard()     {}
func (Echo) wizard()       {}
func (Users) wizard()      {}
func (Moderate) wizard()   {}
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
