package telnet

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

// Render turns anything sent to a connection into the text a telnet client
// sees. It is the single chokepoint between the game's event vocabulary and
// the wire, so keep game logic out of it.
//
// render.self is the name of the player this connection belongs to. Events that the
// whole room sees arrive here once per player, and the case compares Actor to
// self to pick between "Dropped." and "bob drops a knife." -- which is why
// there is no separate notification type for them.
func render(msg any, self string) string {
	switch m := msg.(type) {
	case string: // raw transport text, greetings, login questions, goodbyes ...
		return m

	case event.Prompt:
		hp := fmt.Sprintf("%d/%d", m.CurrentHealth, m.MaxHealth)
		return "<" + paint(healthColor(m.CurrentHealth, m.MaxHealth), hp) + "hp> "

	case event.Failed:
		return failureText(m.Verb, string(m.Code))

	// ---- session -----------------------------------------------------------
	// The login events are consumed by conn.send before they ever reach the
	// queue; these cases exist so a stray one doesn't print a struct dump.

	case event.LoggedIn, event.PlayerCreated, event.LoginFailed, event.CreateFailed:
		return ""

	case event.LoggedOut:
		return m.Actor + " has logged out.\n"

	case event.EnteredGame:
		return m.Actor + " has entered the game.\n"

	case event.Welcome:
		return m.Text + "\n"

	case event.Color:
		// conn.frame has already switched; this only answers the player
		switch {
		case !m.Changed:
			return ""
		case m.On:
			return "Color is " + paint(colorRoomName, "on") + ".\n"
		}
		return "Color is off.\n"

	case event.Pong:
		return "Pong " + m.Target + ".\n"

	// ---- rooms -------------------------------------------------------------

	case event.RoomDescription:
		return renderRoom(m)

	case event.Entered:
		return m.Who + " enters.\n"

	case event.Left:
		// recall and other magical moves leave in no direction at all
		if m.Direction == rules.DirectionNone {
			return m.Who + " leaves.\n"
		}
		return m.Who + " leaves " + strings.ToLower(m.Direction.String()) + ".\n"

	case event.Exits:
		return renderExits(m.Exits)

	// ---- objects -----------------------------------------------------------

	case event.Dropped:
		if m.Actor == self {
			return "Dropped.\n"
		}
		return m.Actor + " drops " + m.Item + ".\n"

	case event.Got:
		switch {
		case m.From != "" && m.Actor == self:
			return "You get " + m.Item + " from " + m.From + ".\n"
		case m.From != "":
			return m.Actor + " gets " + m.Item + " from " + m.From + ".\n"
		case m.Actor == self:
			return "Taken.\n"
		}
		return m.Actor + " gets " + m.Item + ".\n"

	case event.ContainerContents:
		return renderContainerContents(m)

	case event.Decayed:
		return capitalize(m.Item) + " crumbles to dust.\n"

	case event.Equipped:
		return "Equipped.\n"

	case event.Worn:
		return "Done.\n"

	case event.Removed:
		return "You stop using " + m.Item + ".\n"

	case event.Repaired:
		if m.Cost == 0 {
			return "The smith works on " + m.Item + " until it's as good as new.\n"
		}
		return fmt.Sprintf("The smith works on %s until it's as good as new, for %s.\n", m.Item, coins(m.Cost))

	case event.TooExpensive:
		return fmt.Sprintf("%s would cost %s, and you have %s.\n", capitalize(m.Item), coins(m.Cost), coins(m.Coins))

	case event.Inventory:
		return renderInventory(m.Items) + renderPurse(m.Coins)

	case event.Equipment:
		return renderEquipment(m.Power, m.Items)

	// ---- talking -----------------------------------------------------------

	case event.Said:
		if m.Speaker == self {
			return "You say, \"" + paint(colorSay, m.Value) + "\".\n"
		}
		return m.Speaker + " says, \"" + paint(colorSay, m.Value) + "\".\n"

	case event.Told:
		if m.From == self {
			return "Ok.\n"
		}
		return paint(colorTell, m.From+" tells you, \""+m.Value+"\".") + "\n"

	case event.Shouted:
		if m.Speaker == self {
			return "Ok.\n"
		}
		return paint(colorShout, m.Speaker+" shouts, \""+m.Value+"\".") + "\n"

	// ---- the player --------------------------------------------------------

	case event.Who:
		return renderWho(m.Players)

	case event.Stat:
		return renderPlayerStat(m)

	case event.Role:
		return renderRole(m)

	// ---- combat ------------------------------------------------------------

	case event.Attacking:
		return "Ok.\n"

	case event.Considered:
		return renderConsidered(m)

	case event.GearDamaged:
		if m.Items == 1 {
			return "Dying has taken its toll on your equipment.\n"
		}
		return fmt.Sprintf("Dying has taken its toll on your equipment (%d pieces).\n", m.Items)

	case event.Broke:
		if m.Actor == self {
			return paint(colorBroken, fmt.Sprintf("Your %s gives out, ruined.", m.Item)) + "\n"
		}
		return fmt.Sprintf("%s's %s gives out, ruined.\n", m.Actor, m.Item)

	case event.Struck:
		return renderViolence(self, m)

	case event.Died:
		if m.IsPlayer && m.Target == self {
			return paint(colorDeath, "You are dead!") + "\n"
		}
		return paint(colorDeath, m.Target+" is dead!") + "\n"

	case event.Fleeing:
		if m.Who == self {
			return "You attempt to flee!\n"
		}
		return m.Who + " panics, and attempts to flee!\n"

	case event.FleeAttemptFailed:
		if m.Who == self {
			return "PANIC! You couldn't escape!\n"
		}
		return m.Who + " attempts to flee, but can't!\n"

	case event.Fled:
		if m.Who == self {
			return "You flee head over heels.\n"
		}
		return m.Who + " flees head over heels.\n"

	case event.Restored:
		return m.Target + " is restored!\n"

	// ---- builder commands --------------------------------------------------

	case event.Loaded:
		return "Loaded.\n"

	case event.RoomStatus:
		return renderRoomStatus(m)

	default:
		log.Warn().Msgf("telnet render: no case for %T", msg)
		return fmt.Sprintf("%v\n", m)
	}
}

func renderEquipment(power int, equipment []event.EquippedItem) string {
	if len(equipment) == 0 {
		return "Nothing equipped.\n"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("You are using (power %d):\n", power))
	for _, eq := range equipment {
		// include instance id just for testing, for now...
		b.WriteString(fmt.Sprintf("%s\t%s\t[power %d] %s(%s)\n", eq.Slot, eq.ShortDescription, eq.Power, condition(eq), eq.Id))
	}
	b.WriteString("\n")
	return b.String()
}

// condition is what shape a piece of equipment is in, for the listing. Gear
// that never wears out says nothing at all, so a game with no durability.json
// reads exactly as it did before there was one.
func condition(eq event.EquippedItem) string {
	switch {
	case eq.Broken:
		return "(broken) "
	case eq.MaxDurability > 0:
		return fmt.Sprintf("(%d/%d) ", eq.Durability, eq.MaxDurability)
	default:
		return ""
	}
}

func renderExits(exits []event.Exit) string {
	var b strings.Builder
	b.WriteString("Exits:\n")
	if len(exits) == 0 {
		b.WriteString("None!\n")
	} else {
		var exitStrs []string
		for _, exit := range exits {
			exitStrs = append(exitStrs, strings.ToLower(exit.Direction.String()))
		}
		b.WriteString(strings.Join(exitStrs, ", ") + "\n")
	}
	return b.String()
}

// renderInventory formats a list of inventory items as a string for display
// to a mud client.
func renderInventory(items []event.InventoryItem) string {
	if len(items) == 0 {
		return "You aren't carrying anything.\n"
	}
	var b strings.Builder
	b.WriteString("You are carrying:\n")
	for _, item := range items {
		b.WriteString("\t" + item.ShortDescription + "\n")
	}
	return b.String()
}

// coins is an amount as a player reads it.
func coins(n int) string {
	if n == 1 {
		return "1 coin"
	}
	return fmt.Sprintf("%d coins", n)
}

// renderPurse is the line under the inventory. Nothing at all for an empty
// purse, which is everyone before the economy, and a player shouldn't be told
// about a thing they've never seen.
func renderPurse(n int) string {
	if n == 0 {
		return ""
	}
	return "You have " + coins(n) + ".\n"
}

// renderPlayerStat formats a player's stats as a string for display to a mud
// client.
func renderPlayerStat(s event.Stat) string {
	var b strings.Builder
	b.WriteString("Status:\n")
	b.WriteString("Player:\t" + s.PlayerName + "\n")
	b.WriteString("Lineage:\t" + s.Lineage + "\tRole: " + roleOrNone(s.Role) + "\n")
	b.WriteString(fmt.Sprintf("Power:\t%d\n", s.Power))
	b.WriteString(fmt.Sprintf("Health:\t%d of %d\n", s.CurrentHealth, s.MaxHealth))
	b.WriteString(fmt.Sprintf("Coins:\t%d\n", s.Coins))
	b.WriteString("Location:\t" + player.NewLocation(s.ZoneId, s.RoomId).String() + "\n")
	b.WriteString("\n")
	return b.String()
}

// roleOrNone is what goes where a role name goes when the player's equipment
// doesn't add up to one. "none" rather than a blank, so the line doesn't read
// like something failed to load.
func roleOrNone(name string) string {
	if name == "" {
		return "none"
	}
	return name
}

// renderRole prints the standings for every role, not just the winning one.
// A player who is told "you are a Tank" and nothing else has no way to work
// out what to take off.
func renderRole(r event.Role) string {
	var b strings.Builder
	if r.Current == "" {
		b.WriteString("You aren't wearing anything that argues for a role.\n")
	} else {
		b.WriteString("You are fighting as a " + r.Current + ".\n")
		if r.Description != "" {
			b.WriteString(" " + r.Description + "\n")
		}
	}
	width := 0
	for _, s := range r.Standings {
		width = max(width, len(s.Name))
	}
	for _, s := range r.Standings {
		fmt.Fprintf(&b, "  %-*s %2d", width, s.Name, s.Total)
		if len(s.Sources) > 0 {
			b.WriteString("  (" + strings.Join(s.Sources, ", ") + ")")
		}
		b.WriteString("\n")
	}
	b.WriteString("Change what you're wearing to change your role.\n")
	return b.String()
}

// renderRoom formats a RoomDescription as the classic MUD room block:
// name, description, exits, then contents. Objects and mobs arrive
// as complete sentences (DescriptionOnGround / DescriptionInRoom) and
// print as-is; player names don't, so they get a verb here.
func renderRoom(rd event.RoomDescription) string {
	var b strings.Builder
	b.WriteString(paint(colorRoomName, rd.Name) + "\n")
	if rd.Description != "" {
		b.WriteString(" " + rd.Description + "\n")
	}

	b.WriteString(paint(colorExits, "[ Exits: "+rd.Exits+" ]") + "\n")

	for _, o := range rd.Objects {
		b.WriteString(paint(colorObject, o) + "\n")
	}
	for _, m := range rd.Mobs {
		b.WriteString(paint(colorMob, m) + "\n")
	}
	for _, p := range rd.Players {
		b.WriteString(paint(colorPlayer, p) + " is here.\n")
	}
	return b.String()
}

// renderRoomStatus is the builder's dump of everything in a room.
// Deliberately technical: the audience is someone editing content/.
func renderRoomStatus(rs event.RoomStatus) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Room %s.%s %q\n", rs.ZoneId, rs.Id, rs.Name)
	fmt.Fprintf(&b, "Zone: %s (%s)\n", rs.ZoneName, rs.ZoneId)

	var flags []string
	for _, f := range rs.Flags {
		flags = append(flags, string(f))
	}
	fmt.Fprintf(&b, "Flags: %s\n", strings.Join(flags, ", "))

	for _, ex := range rs.Exits {
		fmt.Fprintf(&b, "  exit %-5s -> %s.%s\n", strings.ToLower(ex.Direction.String()), ex.ZoneId, ex.RoomId)
	}
	for _, p := range rs.Players {
		fmt.Fprintf(&b, "  player %s (%d/%d)\n", p.Name, p.CurrentHealth, p.MaxHealth)
	}
	for _, m := range rs.Mobs {
		fmt.Fprintf(&b, "  mob %s.%s %q (%d/%d) %s\n", m.ZoneId, m.DefinitionId, m.Name, m.CurrentHealth, m.MaxHealth, m.Id)
	}
	for _, i := range rs.Items {
		fmt.Fprintf(&b, "  obj %s.%s %q %s\n", i.ZoneId, i.DefinitionId, i.Name, i.Id)
	}
	return b.String()
}

func renderViolence(self string, s event.Struck) string {
	switch {
	case s.Attacker == self:
		if !s.Hit {
			return fmt.Sprintf("You miss %s.\n", s.Target)
		}
		return fmt.Sprintf("You hit %s for %d damage.\n", s.Target, s.Damage)
	case s.Target == self:
		if !s.Hit {
			return fmt.Sprintf("%s misses you.\n", s.Attacker)
		}
		return paint(colorHurt, fmt.Sprintf("%s hits you for %d damage.", s.Attacker, s.Damage)) + "\n"
	default:
		if !s.Hit {
			return fmt.Sprintf("%s misses %s.\n", s.Attacker, s.Target)
		}
		// don't include damage numbers for the bystanders
		return fmt.Sprintf("%s hits %s.\n", s.Attacker, s.Target)
	}
}

func renderWho(players []event.WhoEntry) string {
	var b strings.Builder
	if len(players) == 0 {
		b.WriteString("There's nobody here!\n")
		return b.String()
	}
	b.WriteString("-- Who Is Here --\n")
	for _, p := range players {
		b.WriteString(whoTitle(p) + " - " + p.RoomName + " - " + p.ZoneName + "\n")
	}
	return b.String()
}

// whoTitle is where a MUD traditionally prints a class. It prints the
// lineage and the role instead, and the role can change between two
// consecutive `who`s if the player swaps their gear in between. Either half
// can be missing -- a player in no role at all is just "alice the Hill Dwarf".
func whoTitle(p event.WhoEntry) string {
	name := p.PlayerName
	if p.Bot {
		name += " [bot]"
	}
	var parts []string
	if p.Lineage != "" {
		parts = append(parts, p.Lineage)
	}
	if p.Role != "" {
		parts = append(parts, p.Role)
	}
	if len(parts) == 0 {
		return name
	}
	return name + " the " + strings.Join(parts, " ")
}

// renderConsidered puts the power gap into words, then gives the numbers. The
// steps follow what the gap does to a fight (rules.PowerHitModifier moves a
// point per two): about level is a coin toss, five below is -2 or worse to
// hit and hitting back hard, and ten is as far as the clamp goes.
func renderConsidered(c event.Considered) string {
	var s string
	switch {
	case c.Delta <= -10:
		s = c.Target + " would kill you without noticing."
	case c.Delta <= -5:
		s = c.Target + " would probably kill you."
	case c.Delta <= -2:
		s = c.Target + " would be a real challenge."
	case c.Delta <= 1:
		s = c.Target + " looks like a fair fight."
	case c.Delta <= 4:
		s = c.Target + " should be easy."
	default:
		s = "You could kill " + c.Target + " with your eyes closed."
	}
	return fmt.Sprintf("%s (power %d; you are %d)\n", capitalize(s), c.TargetPower, c.YourPower)
}

// capitalize the first letter, for a mob name ("field rat") that ends up
// starting a sentence.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

func renderContainerContents(c event.ContainerContents) string {
	if len(c.Items) == 0 && c.Coins == 0 {
		return capitalize(c.Container) + " is empty.\n"
	}
	var b strings.Builder
	b.WriteString(capitalize(c.Container) + " holds:\n")
	if c.Coins > 0 {
		b.WriteString("  " + coins(c.Coins) + "\n")
	}
	for _, item := range c.Items {
		b.WriteString(fmt.Sprintf("  %s [power %d]\n", item.ShortDescription, item.Power))
	}
	return b.String()
}
