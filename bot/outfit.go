package bot

import (
	"regexp"
	"strconv"
	"strings"
)

// piece is one thing a hunter buys at the General Store: its full name, as the
// equipment list shows it; the keyword buy and wear take -- one word, which
// only this piece answers to among the stock before it, the kit and the
// starting gear; the slot as the equipment list shows it; and an alias of the
// starter piece it replaces, a word of its short description, or "" for a
// slot the starting kit leaves empty. TestKit_theStoreSellsIt holds it to the
// real content.
type piece struct{ name, keyword, slot, replaces string }

// kit is what a hunter buys, in order: power 2 after the jerkin, 3 after the
// gloves, and no further -- the temple token is power 1 and stays on.
var kit = []piece{
	{"arming sword", "arming", "wielded", "training"},
	{"quilted jerkin", "quilted", "body", "wool"},
	{"studded cap", "studded", "head", ""},
	{"studded boots", "hobnailed", "feet", ""},
	{"riding gloves", "riding", "hands", ""},
	{"oilskin cloak", "oilskin", "about body", "travelers"},
}

// worn is one row of the equipment list. Durability and maxDurability are both
// zero for gear that never wears out.
type worn struct {
	slot, item                string
	power                     int
	durability, maxDurability int //both 0: never wears out
	broken                    bool
}

var (
	// a row: two spaces in, columns two or more spaces apart, the condition
	// last or not there at all
	equipmentRowRe = regexp.MustCompile(`(?m)^  (\S+(?: \S+)?)  +(.+?)  +power (\d+)(?:  +(?:(broken)|(\d+)/(\d+)))?$`)
	purseRe        = regexp.MustCompile(`(?m)^You have (\d+) coins?\.$`)
	inventoryRe    = regexp.MustCompile(`(?m)^(?:Inventory|You aren't carrying anything\.)$`)
	// what the store says to a buy: bought, at a price, or what it would cost
	buyRe     = regexp.MustCompile(`(?m)^(?:You buy .+ for (\d+) coins?\.|[A-Z].* would cost (\d+) coins?, and you have \d+ coins?\.)$`)
	repairRe  = regexp.MustCompile(`(?m)^(?:The smith works on |[A-Z].* would cost \d+ coins?, and you have |That doesn't need repairing\.$)`)
	removedRe = regexp.MustCompile(`(?m)^You stop using .+\.$`)
	woreRe    = regexp.MustCompile(`(?m)^You wear .+\.$`)
)

func readEquipment(text string) []worn {
	var rows []worn
	for _, m := range equipmentRowRe.FindAllStringSubmatch(text, -1) {
		w := worn{slot: m[1], item: m[2], broken: m[4] != ""}
		w.power, _ = strconv.Atoi(m[3])
		w.durability, _ = strconv.Atoi(m[5])
		w.maxDurability, _ = strconv.Atoi(m[6])
		rows = append(rows, w)
	}
	return rows
}

// needsRepair: broken, or down to a third -- the mark that turns yellow for a
// player.
func (w worn) needsRepair() bool {
	return w.broken || (w.maxDurability > 0 && 3*w.durability <= w.maxDurability)
}

func readPurse(text string) int {
	m := purseRe.FindStringSubmatch(text)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func slotHolds(rows []worn, slot string) (worn, bool) {
	for _, r := range rows {
		if r.slot == slot {
			return r, true
		}
	}
	return worn{}, false
}

// nextPiece is the first piece of the kit not worn whose slot is free or holds
// the starter piece it replaces; nil when there's none.
func nextPiece(rows []worn) *piece {
	for i := range kit {
		p := &kit[i]
		r, taken := slotHolds(rows, p.slot)
		switch {
		case !taken:
			return p
		case strings.HasSuffix(r.item, " "+p.name):
			continue // on already
		case p.replaces != "" && strings.Contains(r.item, p.replaces):
			return p
		}
		// something else is there, and nothing it knows takes it off
	}
	return nil
}

// spares is what a hunter has taken off and gives away: the starter pieces,
// by alias.
func spares() []string {
	var out []string
	for _, p := range kit {
		if p.replaces != "" {
			out = append(out, p.replaces)
		}
	}
	return out
}
