// watchmud-sim fights a simulated player against every mob in content, many
// times at each power, and prints how often the player wins: the numbers a
// tuning pass starts from. Melee only -- see package sim for what it leaves
// out, which matters most for the bosses, whose scripts summon help.
//
//	go run ./cmd/watchmud-sim                       # the starting kit, power 1-10
//	go run ./cmd/watchmud-sim -mob mill/drowned_miller -powers 6-9
//	go run ./cmd/watchmud-sim -kit barrow/barrow_blade,barrow/barrow_plate,barrow/barrow_helm
package main

import (
	"cmp"
	"crypto/rand"
	"flag"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/rs/zerolog"
	"github.com/watchmud/watchmud/dice"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/sim"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fs := flag.NewFlagSet("watchmud-sim", flag.ContinueOnError)
	contentPath := fs.String("content", "./content", "content directory")
	n := fs.Int("n", 1000, "fights per matchup")
	kitFlag := fs.String("kit", "", "what the player wears, zone/id,zone/id,... (default: the starting kit)")
	powersFlag := fs.String("powers", "1-10", "the player's powers, a range like 1-10 or a list like 2,4,6")
	mobFlag := fs.String("mob", "", "one mob, zone/id, in detail (default: every mob, win rates only)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	// the loader warns about every optional file a zone doesn't have
	zerolog.SetGlobalLevel(zerolog.ErrorLevel)

	c, err := loader.LoadContent(os.DirFS(*contentPath))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	kit, err := sim.StartingKit(c)
	if *kitFlag != "" {
		kit, err = sim.KitOf(c, strings.Split(*kitFlag, ","))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit:", err)
		return 2
	}
	powers, err := parsePowers(*powersFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "powers:", err)
		return 2
	}
	mobs := allMobs(c)
	if *mobFlag != "" {
		mobs = nil
		zoneId, id, _ := strings.Cut(*mobFlag, "/")
		if z := c.Zones[zoneId]; z != nil && z.MobileDefinitions[id] != nil {
			mobs = []*mobile.Definition{z.MobileDefinitions[id]}
		}
		if mobs == nil {
			fmt.Fprintf(os.Stderr, "no mob %q (want zone/id)\n", *mobFlag)
			return 2
		}
	}
	var seed [32]byte
	_, _ = rand.Read(seed[:])
	roller := dice.New(seed)

	fmt.Printf("%d fights each; kit: %s\n", *n, kitNames(kit))
	fmt.Println("melee only: no abilities, no summons, no wear, no regeneration")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	if *mobFlag != "" {
		fmt.Fprintln(w, "power\twin\tdraw\trounds\thealth left\t")
		for _, p := range powers {
			s, err := sim.Run(roller, c, kit, p, mobs[0], *n)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%.1f\t%s\t\n", p, pct(s.WinRate()),
				pct(float64(s.Draws)/float64(s.Fights)), s.Rounds(), pct(s.HealthLeft()))
		}
		_ = w.Flush()
		return 0
	}
	header := "mob\tzone\tpower\tac\thp\thits\t"
	for _, p := range powers {
		header += "p" + strconv.Itoa(p) + "\t"
	}
	fmt.Fprintln(w, header)
	for _, def := range mobs {
		row := fmt.Sprintf("%s\t%s\t%d\t%d\t%d\t%s\t", def.Name, def.ZoneId, def.Power, def.AC, def.MaxHealth, def.Damage)
		for _, p := range powers {
			s, err := sim.Run(roller, c, kit, p, def, *n)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			row += pct(s.WinRate()) + "\t"
		}
		fmt.Fprintln(w, row)
	}
	_ = w.Flush()
	return 0
}

// allMobs is every mob a player can fight, by zone and then power.
func allMobs(c *loader.Content) []*mobile.Definition {
	var out []*mobile.Definition
	for _, zid := range slices.Sorted(maps.Keys(c.Zones)) {
		z := c.Zones[zid]
		var here []*mobile.Definition
		for _, d := range z.MobileDefinitions {
			if !d.HasFlag(rules.MobileFlagPlayerCantFight) {
				here = append(here, d)
			}
		}
		slices.SortFunc(here, func(a, b *mobile.Definition) int {
			return cmp.Or(cmp.Compare(a.Power, b.Power), cmp.Compare(a.Id, b.Id))
		})
		out = append(out, here...)
	}
	return out
}

func parsePowers(s string) ([]int, error) {
	if lo, hi, ok := strings.Cut(s, "-"); ok {
		a, err1 := strconv.Atoi(lo)
		b, err2 := strconv.Atoi(hi)
		if err1 != nil || err2 != nil || a < 0 || b < a {
			return nil, fmt.Errorf("%q isn't a range", s)
		}
		var out []int
		for p := a; p <= b; p++ {
			out = append(out, p)
		}
		return out, nil
	}
	var out []int
	for _, f := range strings.Split(s, ",") {
		p, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || p < 0 {
			return nil, fmt.Errorf("%q isn't a power", f)
		}
		out = append(out, p)
	}
	return out, nil
}

func kitNames(k sim.Kit) string {
	var names []string
	for _, d := range k {
		names = append(names, d.Name)
	}
	return strings.Join(names, ", ")
}

func pct(f float64) string { return fmt.Sprintf("%.0f%%", 100*f) }
