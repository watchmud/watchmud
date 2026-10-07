// watchmud-load runs the game in-process on loopback, fills it with bots, and
// measures how long a command takes to come back while they play: a few
// probe characters stand in Temple Square typing "look" once a second and
// time each answer.
//
//	go run ./cmd/watchmud-load -players 60 -duration 2m
//
// It makes its own characters, in its own in-memory world, and has no flag
// for an address on purpose: names are permanent, and characters are never
// made on production. Connections are spread over 127.0.0.x, since the
// server allows only a few from any one address.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/watchmud/watchmud/bot"
	"github.com/watchmud/watchmud/dice"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/memstore"
	"github.com/watchmud/watchmud/server"
	"github.com/watchmud/watchmud/telnet"
	"github.com/watchmud/watchmud/world"
	"golang.org/x/crypto/bcrypt"
)

const (
	password   = "loadtestpassword"
	perAddress = 4 // under the server's cap of five
)

func main() { os.Exit(run()) }

func run() int {
	players := flag.Int("players", 40, "bots playing")
	probes := flag.Int("probes", 3, "characters timing look")
	duration := flag.Duration("duration", time.Minute, "how long to run once everyone is in")
	fast := flag.Bool("fast", false, "bots at test speed, not a person's")
	tick := flag.Duration("tick", time.Second, "the server's pulse")
	contentPath := flag.String("content", "./content", "content directory")
	flag.Parse()
	// the game's own logging is for its operators; here it would drown the report
	zerolog.SetGlobalLevel(zerolog.WarnLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr, err := startGame(ctx, *contentPath, *tick)
	if err != nil {
		log.Printf("watchmud-load: %v", err)
		return 1
	}

	names := make([]string, *players+*probes)
	for i := range names {
		names[i] = nameFor(i)
	}
	local := func(i int) string { return fmt.Sprintf("127.0.0.%d", 2+i/perAddress) }
	if len(names)/perAddress > 250 {
		log.Printf("watchmud-load: %d characters is more than 127.0.0.x can spread", len(names))
		return 2
	}

	log.Printf("creating %d characters", len(names))
	if err := forEach(ctx, len(names), 8, func(i int) error { return create(ctx, addr, local(i), names[i]) }); err != nil {
		log.Printf("watchmud-load: creating: %v", err)
		return 1
	}

	playCtx, endPlay := context.WithCancel(ctx)
	defer endPlay()
	var wg sync.WaitGroup
	var dropped atomic.Int32
	bots := make([]*bot.Adventurer, *players)
	styles := []bot.Style{bot.Wanderer, bot.Explorer, bot.Hunter, bot.Wanderer}
	pace := bot.HumanPace
	if *fast {
		pace = fastPace
	}
	for i := range bots {
		bots[i] = bot.NewAdventurer(bot.AdventurerConfig{
			Name: names[i], Password: password, Siblings: slices.Delete(slices.Clone(names), i, i+1),
			Seed: uint64(i + 1), Pace: pace, Style: styles[i%len(styles)], LocalAddr: local(i),
		})
		wg.Go(func() {
			if err := bots[i].Run(playCtx, addr); err != nil && playCtx.Err() == nil {
				// a dropped bot isn't reconnected: how many were is the finding
				dropped.Add(1)
				log.Printf("%s dropped: %s", names[i], firstLine(err.Error()))
			}
		})
	}

	log.Printf("%d bots playing; probing for %s", *players, *duration)
	var mu sync.Mutex
	var took []time.Duration
	var failed int
	probeCtx, endProbe := context.WithTimeout(playCtx, *duration)
	defer endProbe()
	var pwg sync.WaitGroup
	for j := range *probes {
		i := *players + j
		pwg.Go(func() {
			got, bad := probe(probeCtx, addr, local(i), names[i])
			mu.Lock()
			took, failed = append(took, got...), failed+bad
			mu.Unlock()
		})
	}
	pwg.Wait()
	endPlay()
	wg.Wait()

	report(took, failed, bots, *players, int(dropped.Load()))
	return 0
}

// startGame is the real content on 127.0.0.1:0, its own in-memory store and
// real dice, until ctx is done.
func startGame(ctx context.Context, contentPath string, tick time.Duration) (string, error) {
	content, err := loader.LoadContent(os.DirFS(contentPath))
	if err != nil {
		return "", err
	}
	store := memstore.New()
	var seed [32]byte
	_, _ = rand.Read(seed[:])
	w, err := world.New(content, store, dice.New(seed))
	if err != nil {
		return "", err
	}
	gs := server.New(w, content.Catalog, store)
	gs.SetTickInterval(tick)
	gs.SetPasswordCost(bcrypt.MinCost) // throwaway characters, made by the hundred
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	go func() { _ = telnet.Serve(ctx, ln, gs, content.Catalog) }()
	go func() { _ = gs.Run(ctx) }()
	return ln.Addr().String(), nil
}

// nameFor is the ith character's name: letters only, as names must be.
func nameFor(i int) string {
	return fmt.Sprintf("Load%c%c%c", 'a'+i/676%26, 'a'+i/26%26, 'a'+i%26)
}

// create is the creation conversation, against the game this started.
func create(ctx context.Context, addr, local, name string) error {
	c, err := bot.DialFrom(ctx, addr, local)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := converse(c, []step{
		{`By what name do you wish to be known\? `, name},
		{`Create them\? \(yn\) `, "y"},
		{`Which lineage\? `, "1"},
		{`Choose a password: `, password},
		{`Again: `, password},
		{`\[ Exits: `, "quit"},
	}); err != nil {
		return err
	}
	// gone from the world before anyone logs in as them, or the login is
	// refused as already playing
	return c.ExpectClosed(30 * time.Second)
}

type step struct{ want, send string }

func converse(c *bot.Client, steps []step) error {
	for _, s := range steps {
		if _, err := c.Expect(s.want, 30*time.Second); err != nil {
			return fmt.Errorf("waiting for %q: %w", s.want, err)
		}
		if err := c.Send(s.send); err != nil {
			return err
		}
	}
	return nil
}

// probe logs in and types look once a second until ctx is done, answering
// how long each took to come back and how many never did.
func probe(ctx context.Context, addr, local, name string) (took []time.Duration, failed int) {
	c, err := bot.DialFrom(ctx, addr, local)
	if err != nil {
		return nil, 1
	}
	defer c.Close()
	if err := converse(c, []step{
		{`By what name do you wish to be known\? `, name},
		{`Password: `, password},
	}); err != nil {
		log.Printf("%s: %v", name, err)
		return nil, 1
	}
	if _, err := c.Expect(`\[ Exits: `, 30*time.Second); err != nil {
		return nil, 1
	}
	for ctx.Err() == nil {
		start := time.Now()
		if err := c.Send("look"); err != nil {
			return took, failed + 1
		}
		if _, err := c.Expect(`\[ Exits: [^\]\n]*\]`, 10*time.Second); err != nil {
			failed++
		} else {
			took = append(took, time.Since(start))
		}
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
	}
	_ = c.Send("quit")
	return took, failed
}

func report(took []time.Duration, failed int, bots []*bot.Adventurer, players, dropped int) {
	var total bot.Stats
	for _, b := range bots {
		st := b.Stats()
		total.Steps += st.Steps
		total.Kills += st.Kills
		total.Deaths += st.Deaths
		total.Looted += st.Looted
	}
	fmt.Printf("players %d (%d dropped by the server): %d steps, %d kills, %d deaths, %d looted\n",
		players, dropped, total.Steps, total.Kills, total.Deaths, total.Looted)
	if len(took) == 0 {
		fmt.Printf("look: no answers (%d failed)\n", failed)
		return
	}
	slices.Sort(took)
	at := func(p float64) time.Duration { return took[min(len(took)-1, int(p*float64(len(took))))] }
	fmt.Printf("look: %d answered, %d failed; p50 %s, p95 %s, p99 %s, max %s\n",
		len(took), failed, at(0.50), at(0.95), at(0.99), took[len(took)-1])
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// forEach runs f for 0..n-1, at most at a time, stopping at the first error.
func forEach(ctx context.Context, n, at int, f func(int) error) error {
	sem := make(chan struct{}, at)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var first error
	for i := range n {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			if err := f(i); err != nil {
				mu.Lock()
				first = errors.Join(first, fmt.Errorf("%s: %w", nameFor(i), err))
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return first
}

// fastPace is the bots at test speed: a stress, not a crowd.
var fastPace = bot.Pace{
	Think:  [2]time.Duration{0, 2 * time.Millisecond},
	Walk:   [2]time.Duration{0, 2 * time.Millisecond},
	Quiet:  150 * time.Millisecond,
	Poll:   20 * time.Millisecond,
	Answer: 5 * time.Second,
	Idle:   200 * time.Millisecond,
	Linger: [2]time.Duration{50 * time.Millisecond, 200 * time.Millisecond},
}
