package telnet

import (
	"fmt"
	"strings"
	"time"
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
		s := "<" + paint(healthColor(m.CurrentHealth, m.MaxHealth), hp) + "hp"
		if m.MaxMana > 0 {
			s += fmt.Sprintf(" %d/%dm", m.CurrentMana, m.MaxMana)
		}
		return s + "> "

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
		if m.First {
			return m.Actor + " has entered the game for the first time.\n"
		}
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

	case event.Crumbled:
		return m.Name + " crumbles to dust.\n"

	case event.Decayed:
		return capitalize(m.Item) + " crumbles to dust.\n"

	case event.Equipped:
		// wield is the only command that equips into a named slot
		return "You wield " + m.Item + ".\n"

	case event.Worn:
		return "You wear " + m.Item + ".\n"

	case event.GoldGiven:
		return fmt.Sprintf("%d coins appear in your purse. You have %d.\n", m.Amount, m.Coins)

	case event.Removed:
		return "You stop using " + m.Item + ".\n"

	case event.Repaired:
		if m.Cost == 0 {
			return "The smith works on " + m.Item + " until it's as good as new.\n"
		}
		return fmt.Sprintf("The smith works on %s until it's as good as new, for %s.\n", m.Item, coins(m.Cost))

	case event.ShopList:
		return renderShopList(m.Items)

	case event.Bought:
		return fmt.Sprintf("You buy %s for %s.\n", m.Item, coins(m.Cost))

	case event.Sold:
		return fmt.Sprintf("You sell %s for %s.\n", m.Item, coins(m.Coins))

	case event.Valued:
		return fmt.Sprintf("The shopkeeper would give you %s for %s.\n", coins(m.Coins), m.Item)

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

	case event.Healed:
		return renderHealed(m, self)

	case event.Smote:
		if m.Actor == self {
			return fmt.Sprintf("You smite %s! (%d)\n", m.Target, m.Damage)
		}
		return m.Actor + " smites " + m.Target + "!\n"

	case event.Provoked:
		switch {
		case m.Actor == self && m.Already:
			return m.Target + " is already fighting you.\n"
		case m.Actor == self:
			return "You provoke " + m.Target + ", and it turns on you!\n"
		}
		return m.Actor + " provokes " + m.Target + "!\n"

	case event.Stunned:
		if m.Actor == self {
			return "You stun " + m.Target + "!\n"
		}
		return m.Actor + " stuns " + m.Target + "!\n"

	case event.Staggered:
		return m.Name + " staggers, stunned.\n"

	case event.Assessed:
		return renderAssessed(m, self)

	case event.Warded:
		return renderWarded(m, self)

	case event.WardBroken:
		if m.Name == self {
			return "Your ward shatters.\n"
		}
		return m.Name + "'s ward shatters.\n"

	case event.Received:
		// The only backfill so far is the temple token, so the reason to
		// wear it is recall's; a second backfill item would want its own.
		if m.Worn {
			return "You find " + m.Item + " around your neck.\n"
		}
		return "You find " + m.Item + " in your pack. Wear it to recall.\n"

	case event.Summoned:
		if m.Count == 1 {
			return m.Summoner + " calls up one " + m.Name + "!\n"
		}
		return fmt.Sprintf("%s calls up %d %ss!\n", m.Summoner, m.Count, m.Name)

	case event.Abilities:
		return renderAbilities(m)

	// ---- builder commands --------------------------------------------------

	case event.Loaded:
		return "Loaded.\n"

	case event.Slain:
		if m.Actor == self {
			return "You slay " + m.Target + ".\n"
		}
		return m.Actor + " slays " + m.Target + ".\n"

	case event.NoHassle:
		if m.On {
			return "Aggressive mobs will leave you alone.\n"
		}
		return "Aggressive mobs can see you again.\n"

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

// renderAssessed gives the numbers to whoever cast it; the rest of the room
// only sees them looking.
func renderAssessed(m event.Assessed, self string) string {
	if m.Actor != self {
		return m.Actor + " studies " + m.Target + ".\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "You study %s.\n", m.Target)
	fmt.Fprintf(&b, "  Health %d/%d  AC %d  Power %d  Hits for %s\n",
		m.Health, m.MaxHealth, m.ArmorClass, m.Power, m.Damage)
	switch {
	case m.Fighting == self:
		b.WriteString("  Fighting you\n")
	case m.Fighting != "":
		b.WriteString("  Fighting " + m.Fighting + "\n")
	}
	switch {
	case m.Stunned == 1:
		b.WriteString("  Stunned for 1 more round\n")
	case m.Stunned > 1:
		fmt.Fprintf(&b, "  Stunned for %d more rounds\n", m.Stunned)
	}
	return b.String()
}

func renderWarded(m event.Warded, self string) string {
	switch {
	case m.Actor == self && m.Target == self:
		return fmt.Sprintf("A ward settles over you. (%d)\n", m.Amount)
	case m.Actor == self:
		return fmt.Sprintf("You ward %s. (%d)\n", m.Target, m.Amount)
	case m.Target == self:
		return fmt.Sprintf("%s wards you. (%d)\n", m.Actor, m.Amount)
	case m.Actor == m.Target:
		return m.Actor + " casts ward.\n"
	}
	return m.Actor + " wards " + m.Target + ".\n"
}

func renderHealed(m event.Healed, self string) string {
	wasted := m.Amount == 0
	switch {
	case m.Actor == self && m.Target == self:
		if wasted {
			return "You heal yourself, but you weren't hurt.\n"
		}
		return fmt.Sprintf("You heal yourself. (+%d)\n", m.Amount)
	case m.Actor == self:
		if wasted {
			return fmt.Sprintf("You heal %s, but %s wasn't hurt.\n", m.Target, m.Target)
		}
		return fmt.Sprintf("You heal %s. (+%d)\n", m.Target, m.Amount)
	case m.Target == self:
		if wasted {
			return m.Actor + " heals you, but you weren't hurt.\n"
		}
		return fmt.Sprintf("%s heals you. (+%d)\n", m.Actor, m.Amount)
	case m.Actor == m.Target:
		return m.Actor + " casts heal.\n"
	}
	return m.Actor + " heals " + m.Target + ".\n"
}

func renderAbilities(m event.Abilities) string {
	if len(m.Granted) == 0 {
		return "Nothing you're wearing grants any abilities.\n"
	}
	// every column is as wide as its widest entry, measured before color,
	// so the table lines up with color on or off
	type row struct{ name, mana, cooldown, from string }
	rows := make([]row, len(m.Granted))
	width := func(s string, n int) int { return max(len(s), n) }
	nameW, manaW, coolW, fromW := 0, 0, 0, 0
	for i, a := range m.Granted {
		r := row{
			name:     a.Name,
			mana:     fmt.Sprintf("%d mana", a.Mana),
			cooldown: a.Cooldown.String() + " cooldown",
			from:     fmt.Sprintf("%s (power %d)", a.Item, a.Power),
		}
		rows[i] = r
		nameW, manaW = width(r.name, nameW), width(r.mana, manaW)
		coolW, fromW = width(r.cooldown, coolW), width(r.from, fromW)
	}

	var b strings.Builder
	b.WriteString(paint(colorHeading, "Abilities") + "\n")
	for i, a := range m.Granted {
		r := rows[i]
		ready := paint(colorReady, "ready")
		if a.ReadyIn > 0 {
			ready = paint(colorWaiting, fmt.Sprintf("ready in %ds", int((a.ReadyIn+time.Second-1)/time.Second)))
		}
		b.WriteString("  " + pad(paint(colorAbility, r.name), len(r.name), nameW))
		b.WriteString("  " + strings.Repeat(" ", manaW-len(r.mana)) + r.mana) // numbers right-aligned
		b.WriteString("  " + pad(r.cooldown, len(r.cooldown), coolW))
		b.WriteString("  " + pad(r.from, len(r.from), fromW))
		b.WriteString("  " + ready + "\n")
	}
	return b.String()
}

// renderShopList lines the prices up: the reader is comparing them.
func renderShopList(items []event.ShopEntry) string {
	if len(items) == 0 {
		return "Nothing's for sale here, but the shopkeeper will buy.\n"
	}
	width := 0
	for _, it := range items {
		width = max(width, len(it.Item))
	}
	var b strings.Builder
	b.WriteString("For sale here:\n")
	for _, it := range items {
		fmt.Fprintf(&b, "  %-*s  [power %d]  %s\n", width, it.Item, it.Power, coins(it.Price))
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
		switch {
		case s.Absorbed > 0 && s.Damage == 0:
			// it landed, and nothing got through: not a hurt
			return fmt.Sprintf("%s hits you, but your ward takes it. (%d)\n", s.Attacker, s.Absorbed)
		case s.Absorbed > 0:
			return paint(colorHurt, fmt.Sprintf("%s hits you for %d damage; your ward takes %d.",
				s.Attacker, s.Damage, s.Absorbed)) + "\n"
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
	if len(players) == 0 {
		return "There's nobody here!\n"
	}
	// columns sized to the widest name and title, measured before color
	nameWidth, titleWidth := 0, 0
	for _, p := range players {
		nameWidth = max(nameWidth, len(p.PlayerName))
		titleWidth = max(titleWidth, len(whoTitle(p)))
	}
	var b strings.Builder
	people, bots := 0, 0
	for i, p := range players {
		// the world sends people first and bots after; a heading starts each
		if i == 0 || p.Bot != players[i-1].Bot {
			if i > 0 {
				b.WriteString("\n")
			}
			heading := "Players"
			if p.Bot {
				heading = "Bots"
			}
			b.WriteString(paint(colorHeading, heading) + "\n")
		}
		if p.Bot {
			bots++
		} else {
			people++
		}
		nameColor := colorPlayer
		if p.Bot {
			nameColor = colorBot
		}
		b.WriteString("  " + pad(paint(nameColor, p.PlayerName), len(p.PlayerName), nameWidth))
		b.WriteString("  " + pad(paintTitle(p), len(whoTitle(p)), titleWidth))
		b.WriteString("  " + paint(colorPlace, p.RoomName+", "+p.ZoneName) + "\n")
	}
	b.WriteString("\n" + countOf(people, "player", "players"))
	if bots > 0 {
		b.WriteString(" and " + countOf(bots, "bot", "bots"))
	}
	b.WriteString(" online.\n")
	return b.String()
}

// pad fills s, which shows as width columns once its color is gone, out to
// to columns.
func pad(s string, width, to int) string {
	return s + strings.Repeat(" ", max(to-width, 0))
}

func countOf(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// whoTitle is where a MUD traditionally prints a class. It prints the
// lineage and the role instead, and the role can change between two
// consecutive `who`s if the player swaps their gear in between. Either half
// can be missing -- a player in no role at all is just "Hill Dwarf".
func whoTitle(p event.WhoEntry) string {
	return strings.TrimSpace(p.Lineage + " " + p.Role)
}

// paintTitle is whoTitle with the role in color.
func paintTitle(p event.WhoEntry) string {
	if p.Lineage == "" {
		return paint(colorRole, p.Role)
	}
	if p.Role == "" {
		return p.Lineage
	}
	return p.Lineage + " " + paint(colorRole, p.Role)
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
