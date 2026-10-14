package telnet

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/rules"
)

// spoken is screen-reader mode's rendering of msg, where it differs from
// render's: words where render uses symbols or a picture, and lists that say
// how long they are before they start, so a listener knows whether to sit
// through one. False means render's text serves a screen reader as it is.
//
// It sits beside render rather than inside it so that every wording test
// of render still reads what a sighted player sees.
func spoken(msg any, self string) (string, bool) {
	switch m := msg.(type) {
	case event.Prompt:
		s := fmt.Sprintf("health %d of %d", m.CurrentHealth, m.MaxHealth)
		if m.MaxMana > 0 {
			s += fmt.Sprintf(", mana %d of %d", m.CurrentMana, m.MaxMana)
		}
		return s + ". ", true
	case event.RoomDescription:
		text := render(m, self)
		return strings.Replace(text, "[ Exits: "+m.Exits+" ]", "Exits: "+m.Exits+".", 1), true
	case event.Exits:
		return spokenExits(m.Exits), true
	case event.Map:
		return spokenMap(m), true
	case event.Inventory:
		return counted(len(m.Items), "You carry %s.", "thing", m, self), true
	case event.Equipment:
		return counted(len(m.Items), "You wear %s.", "thing", m, self), true
	case event.Who:
		return counted(len(m.Players), "%s online.", "player", m, self), true
	case event.WhereList:
		return counted(len(m.Players), "%s in "+m.Zone+".", "player", m, self), true
	case event.GroupList:
		return counted(len(m.Members), "%s in your group.", "member", m, self), true
	case event.ShopList:
		return counted(len(m.Items), "%s for sale.", "thing", m, self), true
	case event.Abilities:
		return counted(len(m.Granted), "Your gear grants %s.", "ability", m, self), true
	case event.Stat, event.Assessed:
		return ofWords(render(m, self)), true
	}
	return "", false
}

// counted is render's list with a line before it saying how many it holds.
// An empty list says so itself.
func counted(n int, sentence, noun string, msg any, self string) string {
	text := ofWords(render(msg, self))
	if n == 0 {
		return text
	}
	return capitalize(fmt.Sprintf(sentence, howMany(n, noun))) + "\n" + text
}

func howMany(n int, noun string) string {
	if n == 1 {
		return "one " + noun
	}
	if strings.HasSuffix(noun, "y") && !strings.HasSuffix(noun, "ay") {
		return fmt.Sprintf("%d %sies", n, strings.TrimSuffix(noun, "y"))
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// spokenWay is one exit in words: "north to the Millpond", and whether a
// door stands shut in it.
func spokenWay(d rules.Direction, leads string, closed bool) string {
	s := strings.ToLower(d.String())
	if leads != "" {
		s += " to " + leads
	}
	if closed {
		s += ", through a closed door"
	}
	return s
}

func spokenExits(exits []event.Exit) string {
	if len(exits) == 0 {
		return "There's no way out of here.\n"
	}
	var ways []string
	for _, ex := range exits {
		ways = append(ways, spokenWay(ex.Direction, ex.RoomName, ex.Closed))
	}
	return capitalize(howMany(len(exits), "way")) + " out: " + strings.Join(ways, "; ") + ".\n"
}

// spokenMap is "map" as a list in words, never a picture read out character
// by character: the ways out of here, then each room nearby and how far it
// lies in which directions.
func spokenMap(m event.Map) string {
	var here *event.MapRoom
	var near []event.MapRoom
	for i, r := range m.Rooms {
		if r.Here {
			here = &m.Rooms[i]
		} else {
			near = append(near, r)
		}
	}
	if here == nil {
		return "You can't make out a map of this place.\n"
	}
	var b strings.Builder
	b.WriteString("You are at " + here.Name + ", in " + m.Zone + ".\n")
	var ways []string
	for _, ex := range here.Exits {
		ways = append(ways, spokenWay(ex.Direction, ex.Leads, ex.Closed))
	}
	if len(ways) > 0 {
		b.WriteString("From here: " + strings.Join(ways, "; ") + ".\n")
	}
	if len(near) > 0 {
		b.WriteString(capitalize(howMany(len(near), "room")) + " nearby, on this level:\n")
		for _, r := range near {
			b.WriteString(r.Name + ", " + offset(r.X, r.Y) + ".\n")
		}
	}
	return b.String()
}

// offset says where a room lies from here: "2 north and 1 west".
func offset(x, y int) string {
	var parts []string
	if y > 0 {
		parts = append(parts, fmt.Sprintf("%d north", y))
	} else if y < 0 {
		parts = append(parts, fmt.Sprintf("%d south", -y))
	}
	if x > 0 {
		parts = append(parts, fmt.Sprintf("%d east", x))
	} else if x < 0 {
		parts = append(parts, fmt.Sprintf("%d west", -x))
	}
	return strings.Join(parts, " and ")
}

// No \b before: render has often colored the number, and an escape code's
// closing m is a word character.
var fraction = regexp.MustCompile(`(\d+)/(\d+)(hp|m)?`)

// ofWords reads render's "97/100hp" as "97 of 100 health" -- only in the
// lists and blocks that print them, never across text players typed.
func ofWords(text string) string {
	return fraction.ReplaceAllStringFunc(text, func(f string) string {
		g := fraction.FindStringSubmatch(f)
		s := g[1] + " of " + g[2]
		switch g[3] {
		case "hp":
			s += " health"
		case "m":
			s += " mana"
		}
		return s
	})
}
