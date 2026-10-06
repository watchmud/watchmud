package telnet

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/event"
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

	case event.Following:
		return renderFollowing(m, self)
	case event.Followed:
		if m.Fighting {
			return m.Leader + " leaves " + strings.ToLower(m.Direction.String()) + ", but you can't follow in the middle of a fight.\n"
		}
		if m.Down {
			return m.Leader + " leaves " + strings.ToLower(m.Direction.String()) + ", but you'd have to be on your feet to follow.\n"
		}
		return "You follow " + m.Leader + " " + strings.ToLower(m.Direction.String()) + ".\n"
	case event.OOCSaid:
		return paint(colorOOC, "[ooc] "+m.Speaker+": "+m.Value) + "\n"
	case event.OOCSet:
		if m.On {
			return "You're on the ooc channel.\n"
		}
		return "You've left the ooc channel. 'ooc on' to come back.\n"
	case event.Assisted:
		if m.Actor == self {
			return "You leap to " + m.Member + "'s aid against the " + m.Target + "!\n"
		}
		return m.Actor + " leaps to " + m.Member + "'s aid against the " + m.Target + "!\n"
	case event.AssistSet:
		if m.On {
			return "You'll join your group's fights.\n"
		}
		return "You'll stay out of your group's fights unless you join in.\n"
	case event.GroupList:
		return renderGroupList(m)
	case event.GroupTold:
		if m.Speaker == self {
			return paint(colorTell, "You tell the group, '"+m.Value+"'") + "\n"
		}
		return paint(colorTell, m.Speaker+" tells the group, '"+m.Value+"'") + "\n"

	case event.Snatched:
		return "The " + m.Mob + " snatches up " + m.Item + ".\n"
	case event.Swept:
		return "The " + m.Sweeper + " sweeps up " + m.Item + " and tips it into a barrow.\n"

	case event.Tracked:
		if m.Here {
			return "It's right here.\n"
		}
		return "You find a trail: the " + m.Target + " went " + strings.ToLower(m.Direction.String()) + " from here.\n"
	case event.SplitCoins:
		if m.Actor == self {
			return fmt.Sprintf("You split the coins %d ways: %s each.\n", m.Among, coinWord(m.Each))
		}
		return fmt.Sprintf("%s splits some coins %d ways: you get %s.\n", m.Actor, m.Among, coinWord(m.Each))
	case event.WhereList:
		var rows [][]cell
		for _, p := range m.Players {
			rows = append(rows, []cell{colored(colorPlayer, p.Name), colored(colorPlace, p.Room)})
		}
		return paint(colorHeading, "Players in "+m.Zone) + "\n" + table(rows)
	case event.CommandList:
		return renderCommands()
	case event.TimeOfDay:
		s := fmt.Sprintf("It's %s in Wrathrock, %s: %s.\n", m.Clock, m.Day, m.Part)
		if m.Moon != "" {
			s += "The moon is " + m.Moon + ".\n"
		}
		return s
	case event.MoonWarning:
		return paint(colorWounded, "Full moon tonight. Be careful.") + "\n"
	case event.WimpySet:
		switch {
		case m.At == 0 && m.Changed:
			return "You'll fight to the end.\n"
		case m.At == 0:
			return "Wimpy is off: you'll fight to the end. 'wimpy 20' flees below 20 health.\n"
		}
		return fmt.Sprintf("You'll flee when your health drops below %d.\n", m.At)
	case event.Panicked:
		return "You panic and try to run!\n"
	case event.PositionChanged:
		return renderPositionChanged(m, self)
	case event.Reported:
		return "Thanks -- your " + m.Kind + " is noted, for whoever runs the game.\n"
	case event.ReportList:
		if len(m.Reports) == 0 {
			return "No reports since the last restart.\n"
		}
		var b strings.Builder
		b.WriteString(paint(colorHeading, "Reports") + "\n")
		for _, r := range m.Reports {
			fmt.Fprintf(&b, "  %s %s, %s at %s: %s\n", r.When, r.Kind, r.Player, r.Room, r.Text)
		}
		return b.String()
	case event.Echoed:
		return m.Text + "\n"
	case event.UserList:
		return renderUsers(m)
	case event.Moderated:
		return renderModerated(m, self)
	case event.Purged:
		return fmt.Sprintf("Purged %d mobs and %d things.\n", m.Mobs, m.Objects)
	case event.ZoneWasReset:
		return m.Zone + " is reset.\n"

	case event.Socialized:
		switch self {
		case m.Actor:
			return m.ToActor + "\n"
		case m.Target:
			return m.ToTarget + "\n"
		}
		return m.ToRoom + "\n"
	case event.SocialList:
		return paint(colorHeading, "Socials") + "\n" + wrapWords(m.Names, 76)
	case event.Emoted:
		return m.Actor + " " + m.Text + "\n"
	case event.Whispered:
		return renderWhispered(m, self)
	case event.Toggles:
		return renderToggles(m)
	case event.Toggled:
		if m.On {
			return "You'll hear " + m.Name + " again.\n"
		}
		return "You won't hear " + m.Name + " now. 'toggle' shows what's on.\n"

	case event.Junked:
		if m.Actor == self {
			return "You junk " + m.Item + ". It's gone.\n"
		}
		return m.Actor + " junks " + m.Item + ".\n"
	case event.Quaffed:
		if m.Actor == self {
			return "You quaff " + m.Item + ".\n"
		}
		return m.Actor + " quaffs " + m.Item + ".\n"
	case event.Donated:
		if m.Actor == self {
			return "You donate " + m.Item + ". It's waiting in the donation room.\n"
		}
		return m.Actor + " donates " + m.Item + ".\n"
	case event.Appeared:
		return capitalize(m.Item) + " appears, donated.\n"

	case event.Gave:
		switch self {
		case m.Actor:
			return "You give " + m.Item + " to " + m.Recipient + ".\n"
		case m.Recipient:
			return m.Actor + " gives you " + m.Item + ".\n"
		}
		return m.Actor + " gives " + m.Item + " to " + m.Recipient + ".\n"
	case event.Put:
		if m.Actor == self {
			return "You put " + m.Item + " in " + m.Into + ".\n"
		}
		return m.Actor + " puts " + m.Item + " in " + m.Into + ".\n"

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

	case event.DoorChanged:
		return renderDoorChanged(m, self)

	case event.ContainerChanged:
		verb := doorVerb[m.Change]
		if m.Actor == self {
			return "You " + verb + " the " + m.Container + ".\n"
		}
		return m.Actor + " " + verb + "s the " + m.Container + ".\n"

	case event.LookedAtObject:
		return renderLookedAtObject(m)
	case event.LookedAtMob:
		return renderLookedAtMob(m)
	case event.LookedAtPlayer:
		return renderLookedAtPlayer(m, self)

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
	var rows [][]cell
	for _, eq := range equipment {
		rows = append(rows, []cell{
			plainCell(slotLabel(eq.Slot)),
			colored(colorObject, eq.ShortDescription),
			plainCell(fmt.Sprintf("power %d", eq.Power)),
			wear(eq.Durability, eq.MaxDurability, eq.Broken),
		})
	}
	return paint(colorHeading, "Equipment") + fmt.Sprintf(" (power %d)\n", power) + table(rows)
}

// slotLabel is how a slot reads in the equipment list: where the thing is,
// in words.
func slotLabel(s rules.EquipmentSlot) string {
	switch s {
	case rules.SlotWield:
		return "wielded"
	case rules.SlotHold:
		return "held"
	}
	return strings.ReplaceAll(string(s), "_", " ")
}

// wear is a piece's condition for a listing: broken in red, worn down to a
// third or less in yellow, and nothing at all for gear that never wears out.
func wear(durability, maxDurability int, broken bool) cell {
	switch {
	case broken:
		return colored(colorBroken, "broken")
	case maxDurability <= 0:
		return plainCell("")
	case durability*3 <= maxDurability:
		return colored(colorWounded, fmt.Sprintf("%d/%d", durability, maxDurability))
	}
	return plainCell(fmt.Sprintf("%d/%d", durability, maxDurability))
}

func renderExits(exits []event.Exit) string {
	var b strings.Builder
	b.WriteString("Exits:\n")
	if len(exits) == 0 {
		b.WriteString("None!\n")
	} else {
		var exitStrs []string
		for _, exit := range exits {
			s := strings.ToLower(exit.Direction.String())
			if exit.Closed {
				s += " (closed)"
			}
			exitStrs = append(exitStrs, s)
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
	// the same thing in the same shape is one line with a count: a pack of
	// goose feathers is "a long goose feather (x5)", not five lines
	type key struct {
		desc               string
		power, dur, maxDur int
		broken             bool
		bag                bool
		holding            int
	}
	var order []key
	count := map[key]int{}
	for _, it := range items {
		k := key{it.ShortDescription, it.Power, it.Durability, it.MaxDurability, it.Broken, it.Bag, it.Holding}
		if count[k] == 0 {
			order = append(order, k)
		}
		count[k]++
	}
	var rows [][]cell
	for _, k := range order {
		desc := k.desc
		if n := count[k]; n > 1 {
			desc += fmt.Sprintf(" (x%d)", n)
		}
		switch {
		case k.bag && k.holding == 0:
			desc += " (empty)"
		case k.bag:
			desc += fmt.Sprintf(" (%d inside)", k.holding)
		}
		rows = append(rows, []cell{
			colored(colorObject, desc),
			plainCell(fmt.Sprintf("power %d", k.power)),
			wear(k.dur, k.maxDur, k.broken),
		})
	}
	return paint(colorHeading, "Inventory") + "\n" + table(rows)
}

// doorVerb is what a change to a lock is, as a verb.
var doorVerb = map[event.DoorChange]string{
	event.DoorOpened: "open", event.DoorClosed: "close",
	event.DoorLocked: "lock", event.DoorUnlocked: "unlock",
}

// renderDoorChanged: the one who did it, the room that watched, and the far
// side, which heard the door but saw nobody.
func renderDoorChanged(m event.DoorChanged, self string) string {
	verb := doorVerb[m.Change]
	switch {
	case m.Actor == self:
		return "You " + verb + " the " + m.Door + ".\n"
	case m.Actor != "":
		return m.Actor + " " + verb + "s the " + m.Door + ".\n"
	}
	where := strings.ToLower(m.Direction.String())
	if m.Direction == rules.DirectionUp || m.Direction == rules.DirectionDown {
		where = map[rules.Direction]string{rules.DirectionUp: "above", rules.DirectionDown: "below"}[m.Direction]
	} else {
		where = "to the " + where
	}
	switch m.Change {
	case event.DoorOpened:
		return capitalize("the "+m.Door+" "+where+" opens.") + "\n"
	case event.DoorClosed:
		return capitalize("the "+m.Door+" "+where+" closes.") + "\n"
	}
	return "You hear a click from the " + m.Door + " " + where + ".\n"
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
	var rows [][]cell
	for _, a := range m.Granted {
		ready := colored(colorReady, "ready")
		if a.ReadyIn > 0 {
			ready = colored(colorWaiting, fmt.Sprintf("ready in %ds", int((a.ReadyIn+time.Second-1)/time.Second)))
		}
		rows = append(rows, []cell{
			colored(colorAbility, a.Name),
			number(fmt.Sprintf("%d mana", a.Mana)),
			plainCell(seconds(a.Cooldown) + " cooldown"),
			plainCell(fmt.Sprintf("%s (power %d)", a.Item, a.Power)),
			ready,
		})
	}
	return paint(colorHeading, "Abilities") + "\n" + table(rows)
}

// seconds is a cooldown as a player reads it: "60s", not Go's "1m0s".
func seconds(d time.Duration) string {
	return fmt.Sprintf("%ds", int((d+time.Second-1)/time.Second))
}

// renderShopList lines the prices up: the reader is comparing them.
func renderShopList(items []event.ShopEntry) string {
	if len(items) == 0 {
		return "Nothing's for sale here, but the shopkeeper will buy.\n"
	}
	var rows [][]cell
	for _, it := range items {
		rows = append(rows, []cell{
			colored(colorObject, it.Item),
			plainCell(fmt.Sprintf("power %d", it.Power)),
			{text: coins(it.Price), color: colorCoins, right: true},
		})
	}
	return paint(colorHeading, "For sale") + "\n" + table(rows)
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
	return "You have " + paint(colorCoins, coins(n)) + ".\n"
}

// renderPlayerStat formats a player's stats as a string for display to a mud
// client.
func renderPlayerStat(s event.Stat) string {
	title := paint(colorPlayer, s.PlayerName) + ", the " + s.Lineage
	if s.Role != "" {
		title += " " + paint(colorRole, s.Role)
	}
	rows := [][]cell{
		{plainCell("Health"), colored(healthColor(s.CurrentHealth, s.MaxHealth), fmt.Sprintf("%d/%d", s.CurrentHealth, s.MaxHealth))},
		{plainCell("Mana"), plainCell(fmt.Sprintf("%d/%d", s.CurrentMana, s.MaxMana))},
		{plainCell("Power"), plainCell(fmt.Sprint(s.Power))},
		{plainCell("Armor class"), plainCell(fmt.Sprint(s.ArmorClass))},
		{plainCell("Coins"), colored(colorCoins, fmt.Sprint(s.Coins))},
		{plainCell("Where"), colored(colorPlace, s.RoomName+", "+s.ZoneName)},
	}
	return title + "\n" + table(rows)
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
		b.WriteString("You are fighting as a " + paint(colorRole, r.Current) + ".\n")
		if r.Description != "" {
			b.WriteString(" " + r.Description + "\n")
		}
	}
	var rows [][]cell
	for _, st := range r.Standings {
		name := plainCell(st.Name)
		if st.Name == r.Current {
			name = colored(colorRole, st.Name)
		}
		row := []cell{name, number(fmt.Sprint(st.Total))}
		if len(st.Sources) > 0 {
			row = append(row, plainCell("("+strings.Join(st.Sources, ", ")+")"))
		}
		rows = append(rows, row)
	}
	b.WriteString(table(rows))
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
	for i, p := range rd.Players {
		how := ""
		if i < len(rd.PlayerPositions) && rd.PlayerPositions[i] != "" {
			how = rd.PlayerPositions[i] + " "
		}
		b.WriteString(paint(colorPlayer, p) + " is " + how + "here.\n")
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

// renderLookedAtObject is what one thing is: where it goes, what it's made of,
// its power and condition, and what it does.
func renderLookedAtObject(m event.LookedAtObject) string {
	var b strings.Builder
	b.WriteString(paint(colorObject, capitalize(m.Item)) + "\n")
	var kind string
	switch {
	case m.ArmorType != rules.ArmorTypeNone:
		kind = capitalize(string(m.ArmorType)) + " armor, worn on the " + slotLabel(m.Slot)
	case m.Slot == rules.SlotWield:
		kind = "A weapon"
	case m.Slot == rules.SlotHold:
		kind = "Held in the hand"
	case m.Slot != rules.SlotNone && m.Slot != "":
		kind = "Worn on the " + slotLabel(m.Slot)
	}
	if kind != "" {
		fmt.Fprintf(&b, "  %s. Power %d.\n", kind, m.Power)
	} else {
		fmt.Fprintf(&b, "  Power %d.\n", m.Power)
	}
	if c := wear(m.Durability, m.MaxDurability, m.Broken); c.text != "" {
		text := c.text
		if c.color != "" {
			text = paint(c.color, text)
		}
		b.WriteString("  Condition " + text + ".\n")
	}
	if m.Damage != "" {
		fmt.Fprintf(&b, "  Hits for %s.\n", m.Damage)
	}
	if m.Armor > 0 {
		fmt.Fprintf(&b, "  Adds %d to armor class.\n", m.Armor)
	}
	if len(m.Abilities) > 0 {
		b.WriteString("  Lets you cast " + strings.ToLower(strings.Join(m.Abilities, ", ")) + ".\n")
	}
	switch {
	case m.Locked:
		b.WriteString("  It's locked.\n")
	case m.Closed:
		b.WriteString("  It's closed.\n")
	case m.Container && m.Capacity > 0:
		fmt.Fprintf(&b, "  Holding %d of %d.\n", m.Holding, m.Capacity)
	case m.Container && m.Holding == 0:
		b.WriteString("  It's empty.\n")
	case m.Container:
		fmt.Fprintf(&b, "  Holding %d.\n", m.Holding)
	}
	if m.Worn {
		b.WriteString("  You have it on.\n")
	}
	return b.String()
}

// healthWords is how hurt something looks, from a percentage of its health.
func healthWords(pct int) string {
	switch {
	case pct >= 100:
		return "in perfect health"
	case pct >= 75:
		return "slightly hurt"
	case pct >= 50:
		return "wounded"
	case pct >= 25:
		return "badly hurt"
	}
	return "near death"
}

func renderLookedAtMob(m event.LookedAtMob) string {
	s := "The " + m.Name + " is " + healthWords(m.Health)
	if m.Fighting != "" {
		s += ", fighting " + m.Fighting
	}
	return s + ".\n"
}

func renderLookedAtPlayer(m event.LookedAtPlayer, self string) string {
	var b strings.Builder
	who := paint(colorPlayer, m.Name) + " is"
	if m.Name == self {
		who = "You are"
	}
	fmt.Fprintf(&b, "%s %s %s, %s", who, article(m.Lineage), m.Lineage, healthWords(m.Health))
	if m.Fighting != "" {
		b.WriteString(", fighting " + m.Fighting)
	}
	b.WriteString(".\n")
	if m.Role != "" {
		fmt.Fprintf(&b, "  Geared as a %s.\n", m.Role)
	}
	if len(m.Wearing) == 0 {
		b.WriteString("  Wearing nothing at all.\n")
	} else {
		b.WriteString("  Wearing " + strings.Join(m.Wearing, ", ") + ".\n")
	}
	return b.String()
}

// article is "a" or "an" for a word, by its first letter.
func article(word string) string {
	if word != "" && strings.ContainsRune("AEIOUaeiou", rune(word[0])) {
		return "an"
	}
	return "a"
}

func renderFollowing(m event.Following, self string) string {
	switch {
	case self == m.Follower && m.Stopped:
		return "You stop following " + m.Leader + ".\n"
	case self == m.Follower:
		return "You now follow " + m.Leader + ".\n"
	case m.Stopped:
		return m.Follower + " stops following you.\n"
	}
	return m.Follower + " now follows you.\n"
}

// renderGroupList is "group": who, how they're doing, and where.
func renderGroupList(m event.GroupList) string {
	var rows [][]cell
	for _, p := range m.Members {
		name := colored(colorPlayer, p.Name)
		if p.Leader {
			name = colored(colorPlayer, p.Name+" (leader)")
		}
		rows = append(rows, []cell{
			name,
			number(fmt.Sprintf("%d/%dhp", p.Health, p.MaxHealth)),
			number(fmt.Sprintf("%d/%dm", p.Mana, p.MaxMana)),
			colored(colorPlace, p.Room),
		})
	}
	return paint(colorHeading, "Group") + "\n" + table(rows)
}

// wrapWords lays words out in lines of at most width, two spaces in.
func wrapWords(words []string, width int) string {
	var b strings.Builder
	line := " "
	for _, w := range words {
		if len(line)+1+len(w) > width {
			b.WriteString(line + "\n")
			line = " "
		}
		line += " " + w
	}
	if strings.TrimSpace(line) != "" {
		b.WriteString(line + "\n")
	}
	return b.String()
}

func renderWhispered(m event.Whispered, self string) string {
	verb, past := "whisper to", "whispers to"
	if m.Ask {
		verb, past = "ask", "asks"
	}
	switch self {
	case m.From:
		return "You " + verb + " " + m.To + ", '" + m.Value + "'\n"
	case m.To:
		return m.From + " " + past + " you, '" + m.Value + "'\n"
	}
	if m.Ask {
		return m.From + " asks " + m.To + " something.\n"
	}
	return m.From + " whispers something to " + m.To + ".\n"
}

func renderToggles(m event.Toggles) string {
	onOff := func(on bool) cell {
		if on {
			return colored(colorReady, "on")
		}
		return plainCell("off")
	}
	return paint(colorHeading, "Toggles") + "\n" + table([][]cell{
		{plainCell("color"), onOff(m.Color)},
		{plainCell("ooc"), onOff(m.OOC)},
		{plainCell("tells"), onOff(m.Tells)},
		{plainCell("shouts"), onOff(m.Shouts)},
		{plainCell("assist"), onOff(m.Assist)},
	}) + "  'toggle <name>' switches one.\n"
}

func renderUsers(m event.UserList) string {
	var rows [][]cell
	for _, u := range m.Users {
		var flags []string
		for _, f := range []struct {
			on   bool
			name string
		}{{u.Wizard, "wizard"}, {u.Bot, "bot"}, {u.Muted, "muted"}, {u.Frozen, "frozen"}} {
			if f.on {
				flags = append(flags, f.name)
			}
		}
		rows = append(rows, []cell{colored(colorPlayer, u.Name), colored(colorPlace, u.Room), plainCell(u.Zone), plainCell(strings.Join(flags, " "))})
	}
	return paint(colorHeading, fmt.Sprintf("Users (%d)", len(m.Users))) + "\n" + table(rows)
}

func renderModerated(m event.Moderated, self string) string {
	what := map[[2]bool]string{
		{false, true}: "muted", {false, false}: "unmuted",
		{true, true}: "frozen", {true, false}: "thawed",
	}[[2]bool{m.Freeze, m.On}]
	if m.Target == self {
		return "You have been " + what + ".\n"
	}
	return m.Target + " is " + what + ".\n"
}

func renderPositionChanged(m event.PositionChanged, self string) string {
	you := m.Actor == self
	pick := func(mine, theirs string) string {
		if you {
			return mine + "\n"
		}
		return m.Actor + " " + theirs + "\n"
	}
	switch m.To {
	case "sitting":
		return pick("You sit down.", "sits down.")
	case "resting":
		return pick("You sit back and rest.", "sits back to rest.")
	case "sleeping":
		return pick("You lie down and go to sleep.", "lies down and goes to sleep.")
	}
	if m.Woke {
		return pick("You wake and get to your feet.", "wakes and gets up.")
	}
	return pick("You stand up.", "stands up.")
}

func coinWord(n int) string {
	if n == 1 {
		return "1 coin"
	}
	return fmt.Sprintf("%d coins", n)
}

// renderCommands is every verb help knows, in help's order, each once.
func renderCommands() string {
	var verbs []string
	seen := map[string]bool{}
	for _, section := range helpSections {
		for _, e := range section.entries {
			for _, v := range e.verbs {
				if !seen[v] && len(v) > 1 {
					seen[v] = true
					verbs = append(verbs, v)
				}
			}
		}
	}
	return paint(colorHeading, "Commands") + "\n" + wrapWords(verbs, 76) + "  'help' says what they do.\n"
}
