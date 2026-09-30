// watchmud-bots plays the bots: one adventurer per name in WATCHMUD_BOTS, all
// sharing WATCHMUD_BOTS_PASSWORD, each logging back in after anything that
// knocks it off -- a deploy, a crash, a bad connection -- forever.
//
//	WATCHMUD_BOTS=Wren,Pim,Odo WATCHMUD_BOTS_PASSWORD=... watchmud-bots -addr watchmud:4000
//
// The characters are made by hand and flagged as bots on their records
// (deploy/README.md, "Bots"); this never creates one.
package main

import (
	"context"
	"flag"
	"hash/fnv"
	"log"
	"math/rand/v2"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/watchmud/watchmud/bot"
)

func main() {
	os.Exit(run())
}

func run() int {
	addr := flag.String("addr", "localhost:4000", "the game's telnet address")
	flag.Parse()
	log.SetFlags(log.LstdFlags)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	names := splitNames(os.Getenv("WATCHMUD_BOTS"))
	password := os.Getenv("WATCHMUD_BOTS_PASSWORD")
	if len(names) == 0 || password == "" {
		// idle rather than exit, so restart: unless-stopped doesn't spin
		log.Print("watchmud-bots: no bots configured (WATCHMUD_BOTS, WATCHMUD_BOTS_PASSWORD); idling")
		<-ctx.Done()
		return 0
	}
	if len(names) > bot.MaxBots {
		log.Printf("watchmud-bots: %d bots, but the server allows %d connections from one address", len(names), bot.MaxBots)
		return 2
	}

	var wg sync.WaitGroup
	for i, name := range names {
		wg.Go(func() {
			siblings := without(names, name)
			// staggered, so a deploy doesn't bring them all in at once
			if !sleep(ctx, time.Duration(i)*45*time.Second) {
				return
			}
			superviseForever(ctx, *addr, bot.AdventurerConfig{
				Name:     name,
				Password: password,
				Siblings: siblings,
				Seed:     seedFor(name),
				Pace:     bot.HumanPace,
				Log:      log.Printf,
			})
		})
	}
	wg.Wait()
	log.Print("watchmud-bots: stopped")
	return 0
}

// superviseForever runs one adventurer, and after anything that stops it, waits
// 30 seconds to 2 minutes and runs it again.
func superviseForever(ctx context.Context, addr string, cfg bot.AdventurerConfig) {
	for {
		err := bot.NewAdventurer(cfg).Run(ctx, addr)
		if ctx.Err() != nil {
			return
		}
		wait := 30*time.Second + time.Duration(rand.Int64N(int64(90*time.Second)))
		log.Printf("%s: %v; back in %s", cfg.Name, err, wait.Round(time.Second))
		if !sleep(ctx, wait) {
			return
		}
	}
}

func splitNames(s string) []string {
	var out []string
	for _, n := range strings.Split(s, ",") {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, n)
		}
	}
	return out
}

func without(names []string, name string) []string {
	var out []string
	for _, n := range names {
		if n != name {
			out = append(out, n)
		}
	}
	return out
}

// seedFor gives each bot its own dice, the same ones every run.
func seedFor(name string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	return h.Sum64()
}

// sleep waits d, or returns false if ctx ends first.
func sleep(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
