package bot

import (
	"context"
	"regexp"
	"slices"
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
	refusedRe = regexp.MustCompile(`(?m)^[A-Z].* would cost (\d+) coins?, and you have \d+ coins?\.$`)
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

// damaged: anything off new, not just at the mark. Once at the smith, a piece
// worn a little is mended too: the cost is in proportion to the wear.
func (w worn) damaged() bool {
	return w.broken || w.durability < w.maxDurability
}

// repairWord is what the smith is asked for this row: the kit piece's
// keyword, or the alias of the starter piece the kit replaces in its slot;
// "" for anything else -- the token, which never wears, or what it didn't buy.
func (w worn) repairWord() string {
	for _, p := range kit {
		switch {
		case p.slot != w.slot:
		case strings.HasSuffix(w.item, " "+p.name):
			return p.keyword
		case p.replaces != "" && strings.Contains(w.item, p.replaces):
			return p.replaces
		}
	}
	return ""
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

// outfit is a visit to the smith and the store, both off Market Square, and
// then out: once a town visit, never back to town, so a repair it can't
// finish doesn't send it round again.
func (a *Adventurer) outfit(rows []worn, coins int) stateFn {
	return func(ctx context.Context) (stateFn, error) {
		if a.attacked {
			return a.fightThen(a.town), nil
		}
		if a.here == home {
			if _, err := a.walk(ctx, waitIn); err != nil {
				return nil, err
			}
		}
		if a.wantsRepair(rows, coins) {
			if _, err := a.walk(ctx, step{"west", "Smithy"}); err != nil {
				return nil, err
			}
			if err := a.atSmith(rows); err != nil {
				return nil, err
			}
			a.count(func(st *Stats) { st.Repairs++ })
			if _, err := a.walk(ctx, step{"east", "Market Square"}); err != nil {
				return nil, err
			}
			inv, _, err := a.ask("inventory", inventoryRe)
			if err != nil {
				return nil, err
			}
			coins = readPurse(inv)
		}
		if a.affordable(rows, coins) != nil {
			if _, err := a.walk(ctx, step{"east", "General Store"}); err != nil {
				return nil, err
			}
			if err := a.shop(ctx, rows, coins); err != nil {
				return nil, err
			}
			if _, err := a.walk(ctx, step{"west", "Market Square"}); err != nil {
				return nil, err
			}
		}
		_, m, err := a.ask("equipment", powerRe)
		if err != nil {
			return nil, err
		}
		power, _ := strconv.Atoi(m[1])
		return a.setOut(ctx, power)
	}
}

// affordable is the next piece of the kit if the purse covers what it's known
// to cost -- or its price isn't known yet, and asking is how to learn it.
func (a *Adventurer) affordable(rows []worn, coins int) *piece {
	p := nextPiece(rows)
	if p == nil {
		return nil
	}
	if price, known := a.prices[p.name]; known && coins < price {
		return nil
	}
	return p
}

// shop is at the General Store: it buys down the kit while the purse lasts,
// takes off the starter piece in the way, and wears what it bought. A refusal
// ends it, with the price learned.
func (a *Adventurer) shop(ctx context.Context, rows []worn, coins int) error {
	for {
		p := a.affordable(rows, coins)
		if p == nil {
			return nil
		}
		if err := a.pause(ctx, a.cfg.Pace.Think); err != nil {
			return err
		}
		_, m, err := a.ask("buy "+p.keyword, buyRe)
		if err != nil {
			return err
		}
		if m[1] == "" {
			a.prices[p.name], _ = strconv.Atoi(m[2])
			a.log("%s costs %d; saving up", p.name, a.prices[p.name])
			return nil
		}
		price, _ := strconv.Atoi(m[1])
		a.prices[p.name] = price
		coins -= price
		if _, taken := slotHolds(rows, p.slot); taken {
			if _, _, err := a.ask("remove "+p.replaces, removedRe); err != nil {
				return err
			}
		}
		if _, _, err := a.ask("wear "+p.keyword, woreRe); err != nil {
			return err
		}
		// on now: what was in its slot is off, so the loop moves down the kit
		rows = append(slices.DeleteFunc(slices.Clone(rows), func(r worn) bool { return r.slot == p.slot }),
			worn{slot: p.slot, item: "a " + p.name, power: 4})
		a.count(func(st *Stats) { st.Bought++ })
		a.log("bought and wore %s for %d coins", p.name, price)
	}
}

// atSmith is at the smith: each worn piece that's seen wear, asked for by the
// word only it answers to -- never "repair all", which would mend the starter
// pieces in the pack too, on their way to the donation room. The cheapest
// refusal is remembered (repairShort), and none clears it.
func (a *Adventurer) atSmith(rows []worn) error {
	a.repairShort = 0
	for _, r := range rows {
		word := r.repairWord()
		if word == "" || !r.damaged() {
			continue
		}
		said, _, err := a.ask("repair "+word, repairRe)
		if err != nil {
			return err
		}
		if n := cheapestRefused(said); n > 0 && (a.repairShort == 0 || n < a.repairShort) {
			a.repairShort = n
		}
	}
	return nil
}

// wantsRepair: something worn is at the mark, and there's a coin to pay with
// -- and at least what the smith last refused, so a bot short of a repair
// doesn't walk over every visit to be told so again.
func (a *Adventurer) wantsRepair(rows []worn, coins int) bool {
	if coins == 0 || coins < a.repairShort {
		return false
	}
	for _, r := range rows {
		if r.needsRepair() {
			return true
		}
	}
	return false
}

// cheapestRefused is the least the smith said a repair it wouldn't do would
// cost, or 0 if it refused nothing.
func cheapestRefused(text string) int {
	least := 0
	for _, m := range refusedRe.FindAllStringSubmatch(text, -1) {
		n, _ := strconv.Atoi(m[1])
		if least == 0 || n < least {
			least = n
		}
	}
	return least
}
