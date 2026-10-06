package bot

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// An adventurer's rules of thumb. Constants rather than config: they are what
// makes a bot polite, and a bot shouldn't be one flag away from rude.
const (
	// MaxBots: the server allows 5 connections per address, and every bot run
	// by one watchmud-bots shares its address.
	MaxBots = 5

	donateAfter = 5                // things looted before a trip to the donation room, unless configured
	avoidFor    = 10 * time.Minute // a ground a player is using is theirs this long
	tellEvery   = 10 * time.Minute // one honest answer per person this often
	talkEvery   = 10 * time.Minute // at most one remark this often
	talkOneIn   = 5                // and on only one moment in this many
	restBelow   = 50               // percent health: stop and rest
	restUntil   = 90               // percent health: rested
	fleeBelow   = 25               // percent health, mid-fight: run
	patrolLaps  = 3                // laps of a ground before heading back to town

	botReply = "I'm a bot (see 'help bots') - I can't chat, sorry!"
)

// home is where recall lands -- and where every death and every new character
// lands too, so it's no place for bots to stand around. waitIn is where one
// with nowhere to hunt waits instead: a step south, and the first room of
// every route, since every ground is out through the market.
var (
	home   = "Temple Square"
	waitIn = step{"south", "Market Square"}
)

// Pace is how fast a bot plays. HumanPace is the real one; tests go faster.
type Pace struct {
	Think  [2]time.Duration // before each decision, uniformly in [min, max)
	Walk   [2]time.Duration // before each step
	Quiet  time.Duration    // a fight is over after this long with no blows
	Poll   time.Duration    // how often a resting bot looks at its health
	Answer time.Duration    // how long the server has to reply
	Idle   time.Duration    // how long a bot with nowhere to hunt waits in town
	Linger [2]time.Duration // how long a wanderer stops in a room, when it does
}

// HumanPace is a person at a keyboard.
var HumanPace = Pace{
	Think:  [2]time.Duration{2 * time.Second, 6 * time.Second},
	Walk:   [2]time.Duration{time.Second, 3 * time.Second},
	Quiet:  4 * time.Second,
	Poll:   5 * time.Second,
	Answer: 10 * time.Second,
	Idle:   3 * time.Minute,
	Linger: [2]time.Duration{30 * time.Second, 2 * time.Minute},
}

// AdventurerConfig is one bot. Siblings are the other bots run beside it:
// never players to make way for, never answered.
type AdventurerConfig struct {
	Name, Password string
	Siblings       []string
	Seed           uint64
	Pace           Pace
	// DonateAfter is how many things it loots before a trip to the donation
	// room; zero is donateAfter. Tests lower it: a world at a hundred times
	// speed still respawns on the wall clock, so its fields run dry first.
	DonateAfter int
	Style       Style
	Log         func(format string, args ...any) // nil is silent
	// LocalAddr is the address to connect from, "" for any; see DialFrom.
	LocalAddr string
}

// Style is what a bot does with its time. Everything else -- reading the
// world, answering tells, fighting back, resting, recall, dying -- they share.
type Style int

const (
	// Hunter goes out to a hunting ground, fights what's fair, loots its
	// kills and gives them to the donation room.
	Hunter Style = iota
	// Wanderer roams wherever the exits go and is safe, fights only what
	// attacks it, and loots nothing (wanderer.go).
	Wanderer
	// Socialite stays in Temple Square, greets new characters and answers
	// questions (socialite.go).
	Socialite
	// Explorer maps the world, taking every exit it may, then wanders; safe
	// the way a wanderer is (explorer.go).
	Explorer
)

// Stats is what an adventurer has done since it started.
type Stats struct {
	Kills, Looted, Donations, TellsAnswered, Avoided, Deaths, Steps int
	// Mapped is how many rooms an explorer has on its atlas.
	Mapped int
}

// Adventurer is a bot that plays like a player: out to a hunting ground, fights
// what it can, loots its own kills, rests when hurt, gives what it finds to the
// donation room. Everything but Stats belongs to the goroutine running Run.
type Adventurer struct {
	cfg AdventurerConfig
	rng *rand.Rand
	now func() time.Time // for what it remembers: tells, avoided grounds, remarks
	c   *Client

	health, maxHealth int
	attacked, died    bool
	carrying          int    // looted since the last donation
	here              string // the room it's in, as far as it knows
	ground            *ground
	avoiding          map[string]time.Time // ground name -> until
	toldAt            map[string]time.Time
	pendingTells      []tell
	saidAt            time.Time
	greeted           map[string]bool // new characters a socialite has welcomed
	atlas             *atlas          // what an explorer has mapped

	mu    sync.Mutex
	stats Stats
}

func NewAdventurer(cfg AdventurerConfig) *Adventurer {
	return &Adventurer{
		cfg:      cfg,
		rng:      rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^0x9e3779b97f4a7c15)),
		now:      time.Now,
		atlas:    newAtlas(),
		avoiding: map[string]time.Time{},
		toldAt:   map[string]time.Time{},
	}
}

// Stats is safe to call while Run runs.
func (a *Adventurer) Stats() Stats {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stats
}

func (a *Adventurer) count(f func(*Stats)) {
	a.mu.Lock()
	f(&a.stats)
	a.mu.Unlock()
}

func (a *Adventurer) log(format string, args ...any) {
	if a.cfg.Log != nil {
		a.cfg.Log(a.cfg.Name+": "+format, args...)
	}
}

// Not the connection's fault, so not Run's to return: each sends the
// adventurer somewhere to start over.
var (
	errLost = errors.New("lost")                       // an answer it didn't expect: recall
	errDied = errors.New("died")                       // wakes in Temple Square
	errBusy = errors.New("in a fight it didn't start") // fight, then recall
)

var (
	okRe             = regexp.MustCompile(`(?m)^Ok\.$`)
	tellAnswerRe     = regexp.MustCompile(`(?m)^Ok\.$|No one by that name is playing\.`)
	anyRe            = regexp.MustCompile(``)
	busyText         = "You're too busy fighting!"
	killRe           = regexp.MustCompile(`(?m)^Ok\.$|You don't see that here\.|You're already fighting!`)
	considerOrGoneRe = regexp.MustCompile(`\(power (\d+); you are (\d+)\)|You don't see that here\.`)
	lootRe           = regexp.MustCompile(`You get |There's nothing in there\.|You don't see that here\.`)
	dropRe           = regexp.MustCompile(`Dropped\.|You aren't carrying that\.`)
	saidRe           = regexp.MustCompile(`You say, "`)
)

// roomRe matches a room description by its name, at the start of a line.
func roomRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `$`)
}

type stateFn func(ctx context.Context) (stateFn, error)

// Run connects, logs in and adventures until ctx is done -- then it quits
// cleanly and returns nil -- or the connection fails, and it returns why, for
// whoever runs it to log it and try again later.
func (a *Adventurer) Run(ctx context.Context, addr string) error {
	dctx, cancel := context.WithTimeout(ctx, time.Minute)
	c, err := DialFrom(dctx, addr, a.cfg.LocalAddr)
	cancel()
	if err != nil {
		return err
	}
	defer c.Close()
	a.c = c
	if _, err := c.Expect(`Welcome to WatchMUD`, a.cfg.Pace.Answer); err != nil {
		return err
	}
	if err := login(c, Config{Name: a.cfg.Name, Password: a.cfg.Password}); err != nil {
		return err
	}
	a.log("logged in")

	state := stateFn(a.goHome)
	for {
		next, err := state(ctx)
		switch {
		case ctx.Err() != nil:
			return a.quit()
		case errors.Is(err, errDied):
			next = a.dead
		case errors.Is(err, errBusy):
			next = a.fightThen(a.goHome)
		case errors.Is(err, errLost):
			a.log("%v; recalling", err)
			next = a.goHome
		case err != nil:
			return err
		}
		state = next
	}
}

func (a *Adventurer) quit() error {
	if err := a.c.Send("quit"); err != nil {
		return nil // already gone
	}
	_ = a.c.ExpectClosed(a.cfg.Pace.Answer)
	a.log("quit")
	return nil
}

// ---- states -----------------------------------------------------------------

// goHome recalls to Temple Square.
func (a *Adventurer) goHome(ctx context.Context) (stateFn, error) {
	if a.attacked {
		return a.fightThen(a.goHome), nil
	}
	if err := a.pause(ctx, a.cfg.Pace.Think); err != nil {
		return nil, err
	}
	if err := a.recall(); err != nil {
		return nil, err
	}
	a.say(momentTown)
	switch a.cfg.Style {
	case Wanderer:
		return a.wander, nil
	case Socialite:
		return a.socialize, nil
	case Explorer:
		return a.explore, nil
	}
	return a.town, nil
}

// town rests if it must, then picks a ground its power suits.
func (a *Adventurer) town(ctx context.Context) (stateFn, error) {
	_, m, err := a.ask("equipment", powerRe)
	if err != nil {
		return nil, err
	}
	if a.needsRest() {
		if err := a.rest(ctx); err != nil {
			return nil, err
		}
		if a.died {
			return a.dead, nil
		}
	}
	power, _ := strconv.Atoi(m[1])
	a.ground = a.pickGround(power)
	if a.ground == nil {
		a.log("nowhere to hunt at power %d; waiting in town", power)
		if a.here == home {
			if _, err := a.walk(ctx, waitIn); err != nil {
				return nil, err
			}
		}
		return a.waitThen(a.cfg.Pace.Idle, a.town), nil
	}
	a.log("off to %s at power %d", a.ground.name, power)
	return a.travel, nil
}

// travel sets out from wherever in town it is: home, or partway down the
// route already, waiting in the market.
func (a *Adventurer) travel(ctx context.Context) (stateFn, error) {
	route := a.ground.route
	for i, s := range route {
		if s.room == a.here {
			route = route[i+1:]
			break
		}
	}
	for _, s := range route {
		if _, err := a.walk(ctx, s); err != nil {
			return nil, err
		}
	}
	a.log("hunting %s", a.ground.name)
	return a.hunt, nil
}

// hunt walks the patrol, fighting what's fair, leaving if a player turns up.
func (a *Adventurer) hunt(ctx context.Context) (stateFn, error) {
	g := a.ground
	for range patrolLaps {
		for i, s := range g.patrol {
			room, err := a.walk(ctx, s)
			if err != nil {
				return nil, err
			}
			if who := playersHere(room, a.cfg.Name, a.cfg.Siblings); len(who) > 0 {
				a.avoiding[g.name] = a.now().Add(avoidFor)
				a.count(func(st *Stats) { st.Avoided++ })
				a.log("%s is in %s; leaving %s to them", who[0], s.room, g.name)
				return a.leave(g.homeward(i)), nil
			}
			for _, p := range g.preyIn(room) {
				if err := a.engage(ctx, p); err != nil {
					return nil, err
				}
				if a.died {
					return a.dead, nil
				}
			}
			if a.attacked {
				if err := a.fight(ctx); err != nil {
					return nil, err
				}
				if a.died {
					return a.dead, nil
				}
			}
			if a.needsRest() {
				if err := a.rest(ctx); err != nil {
					return nil, err
				}
				if a.died {
					return a.dead, nil
				}
			}
			if a.carrying >= a.donateAt() {
				return a.donate, nil
			}
		}
	}
	return a.goHome, nil
}

// donate takes what it found to the donation room, east of Temple Square.
func (a *Adventurer) donate(ctx context.Context) (stateFn, error) {
	if a.attacked {
		return a.fightThen(a.donate), nil
	}
	if err := a.recall(); err != nil {
		return nil, err
	}
	if _, err := a.walk(ctx, step{"east", "Donation Room"}); err != nil {
		return nil, err
	}
	for _, kw := range a.ground.loot {
		if err := a.pause(ctx, a.cfg.Pace.Think); err != nil {
			return nil, err
		}
		if _, _, err := a.ask("drop all."+kw, dropRe); err != nil {
			return nil, err
		}
	}
	a.carrying = 0
	a.count(func(st *Stats) { st.Donations++ })
	a.log("left what it found in the donation room")
	a.say(momentDonate)
	if _, err := a.walk(ctx, step{"west", "Temple Square"}); err != nil {
		return nil, err
	}
	return a.town, nil
}

// leave walks off a ground a player has turned up on, back to town.
func (a *Adventurer) leave(way []step) stateFn {
	return func(ctx context.Context) (stateFn, error) {
		for _, s := range way {
			if _, err := a.walk(ctx, s); err != nil {
				return nil, err
			}
		}
		return a.town, nil
	}
}

// dead: it woke in the player-death room at 1hp; recall makes sure where.
func (a *Adventurer) dead(ctx context.Context) (stateFn, error) {
	a.died, a.attacked = false, false
	a.count(func(st *Stats) { st.Deaths++ })
	a.log("died")
	return a.goHome, nil
}

// fightThen fights, then carries on to next.
func (a *Adventurer) fightThen(next stateFn) stateFn {
	return func(ctx context.Context) (stateFn, error) {
		if err := a.fight(ctx); err != nil {
			return nil, err
		}
		if a.died {
			return a.dead, nil
		}
		return next, nil
	}
}

// waitThen reads the world for d, answering tells and fighting back, then
// carries on to next.
func (a *Adventurer) waitThen(d time.Duration, next stateFn) stateFn {
	return func(ctx context.Context) (stateFn, error) {
		if err := a.idle(ctx, d); err != nil {
			return nil, err
		}
		if a.died {
			return a.dead, nil
		}
		if a.attacked {
			return a.fightThen(next), nil
		}
		return next, nil
	}
}

// idle reads the world for d, answering tells, and stops early if it's
// attacked or dies: the caller sees which.
func (a *Adventurer) idle(ctx context.Context, d time.Duration) error {
	end := time.Now().Add(d)
	for time.Now().Before(end) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		ch, err := a.c.ReadChunk(min(time.Until(end), a.cfg.Pace.Poll))
		if err != nil {
			return err
		}
		a.notice(ch)
		if a.died || a.attacked {
			return nil
		}
		if err := a.flushTells(); err != nil {
			return err
		}
	}
	return nil
}

// ---- procedures -------------------------------------------------------------

// walk takes one step and returns the description of the room it reached.
func (a *Adventurer) walk(ctx context.Context, s step) (string, error) {
	if err := a.pause(ctx, a.cfg.Pace.Walk); err != nil {
		return "", err
	}
	text, _, err := a.ask(s.dir, roomRe(s.room))
	if err == nil {
		a.here = s.room
	}
	return text, err
}

// recall goes home, the one way back that works from anywhere -- once its
// cooldown allows: a bot that died twice in a minute waits it out.
func (a *Adventurer) recall() error {
	for range recallTries {
		text, _, err := a.ask("recall", adventurerRecallRe)
		switch {
		case err != nil:
			return err
		case strings.Contains(text, noRecallText):
			return errors.New(noRecallError)
		case strings.Contains(text, notReadyText):
			time.Sleep(recallRetry)
			continue
		}
		a.here = home
		return nil
	}
	return fmt.Errorf("recall still not ready after %d tries", recallTries)
}

// adventurerRecallRe is home, or one of recall's refusals.
var adventurerRecallRe = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(home) + `$|` +
	regexp.QuoteMeta(notReadyText) + `|` + regexp.QuoteMeta(noRecallText))

// engage considers one kind of prey and fights it if it's a fair fight or
// easier: "a real challenge" is for a group, and a bot hasn't got one.
func (a *Adventurer) engage(ctx context.Context, p prey) error {
	if err := a.pause(ctx, a.cfg.Pace.Think); err != nil {
		return err
	}
	_, m, err := a.ask("consider "+p.keyword, considerOrGoneRe)
	if err != nil {
		return err
	}
	if m[1] == "" {
		return nil // gone: it wandered, or somebody got it
	}
	target, _ := strconv.Atoi(m[1])
	you, _ := strconv.Atoi(m[2])
	if target > you+1 {
		return nil
	}
	if !a.attacked {
		if _, _, err := a.ask("kill "+p.keyword, killRe); err != nil {
			return err
		}
	}
	return a.fight(ctx)
}

// fight lasts until Pace.Quiet goes by without a blow. It flees below
// fleeBelow -- ending up somewhere it didn't choose, which is errLost -- and
// afterwards loots the newest corpse if anything it hunts died.
func (a *Adventurer) fight(ctx context.Context) error {
	killed := 0
	last := time.Now()
	for time.Since(last) < a.cfg.Pace.Quiet {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		ch, err := a.c.ReadChunk(a.cfg.Pace.Quiet - time.Since(last))
		if err != nil {
			return err
		}
		a.notice(ch)
		if a.died {
			a.attacked = false
			return nil
		}
		if blows(ch.Text) {
			last = time.Now()
		}
		for _, name := range deaths(ch.Text) {
			if a.ground != nil && a.ground.isPrey(name) {
				killed++
			}
		}
		if strings.Contains(ch.Text, youFled) {
			a.attacked = false
			return fmt.Errorf("%w: fled", errLost)
		}
		if ch.MaxHealth > 0 && a.below(fleeBelow) {
			if err := a.c.Send("flee"); err != nil {
				return err
			}
		}
	}
	a.attacked = false
	if killed == 0 {
		return nil
	}
	a.count(func(st *Stats) { st.Kills += killed })
	a.say(momentKill)
	text, _, err := a.ask("get all from corpse", lootRe)
	if err != nil {
		return err
	}
	n := looted(text)
	a.carrying += n
	a.count(func(st *Stats) { st.Looted += n })
	return nil
}

// rest waits for regen. Regen prints nothing, so it asks for its prompt with a
// bare Enter, which repeats the newest one the server sent.
func (a *Adventurer) rest(ctx context.Context) error {
	a.log("resting at %d/%d", a.health, a.maxHealth)
	a.say(momentRest)
	for a.maxHealth > 0 && a.health*100 < restUntil*a.maxHealth {
		if err := a.pause(ctx, [2]time.Duration{a.cfg.Pace.Poll, a.cfg.Pace.Poll}); err != nil {
			return err
		}
		if _, _, err := a.ask("", anyRe); err != nil {
			return err
		}
		if a.died {
			return nil
		}
		if a.attacked {
			if err := a.fight(ctx); err != nil {
				return err
			}
			if a.died {
				return nil
			}
		}
	}
	return nil
}

// say makes a remark now and then: on one moment in talkOneIn, never twice in
// talkEvery, and only if this bot has phrases.
func (a *Adventurer) say(m moment) {
	lines := phrases[strings.ToLower(a.cfg.Name)][m]
	if len(lines) == 0 || a.rng.IntN(talkOneIn) != 0 {
		return
	}
	if !a.saidAt.IsZero() && a.now().Sub(a.saidAt) < talkEvery {
		return
	}
	a.saidAt = a.now()
	// a failure here shows up in whatever it does next
	_, _, _ = a.ask("say "+lines[a.rng.IntN(len(lines))], saidRe)
}

// ---- talking to the server ---------------------------------------------------

// ask answers any tells first, then exchanges line.
func (a *Adventurer) ask(line string, re *regexp.Regexp) (string, []string, error) {
	if err := a.flushTells(); err != nil {
		return "", nil, err
	}
	return a.exchange(line, re)
}

func (a *Adventurer) flushTells() error {
	for len(a.pendingTells) > 0 {
		t := a.pendingTells[0]
		a.pendingTells = a.pendingTells[1:]
		reply := botReply
		if a.cfg.Style == Socialite {
			reply = answer(t.text)
		}
		if _, _, err := a.exchange("tell "+t.who+" "+reply, tellAnswerRe); err != nil {
			return err
		}
		a.count(func(st *Stats) { st.TellsAnswered++ })
	}
	return nil
}

// exchange sends line and reads chunks, noticing every one, until one matches
// re; it returns that chunk's text and re's submatches. Dying on the way is
// errDied; being told it's too busy fighting is errBusy; no match within
// Pace.Answer is errLost.
func (a *Adventurer) exchange(line string, re *regexp.Regexp) (string, []string, error) {
	if err := a.c.Send(line); err != nil {
		return "", nil, err
	}
	deadline := time.Now().Add(a.cfg.Pace.Answer)
	for {
		left := time.Until(deadline)
		if left <= 0 {
			return "", nil, fmt.Errorf("%w: no answer to %q", errLost, line)
		}
		ch, err := a.c.ReadChunk(left)
		if err != nil {
			return "", nil, err
		}
		if ch.MaxHealth == 0 {
			continue // timed out; the loop's deadline decides
		}
		a.notice(ch)
		if m := re.FindStringSubmatch(ch.Text); m != nil {
			return ch.Text, m, nil
		}
		if a.died {
			return ch.Text, nil, errDied
		}
		if strings.Contains(ch.Text, busyText) {
			return ch.Text, nil, errBusy
		}
	}
}

// notice takes in a chunk: health, tells to answer, being attacked, dying.
func (a *Adventurer) notice(ch Chunk) {
	if ch.MaxHealth > 0 {
		a.health, a.maxHealth = ch.Health, ch.MaxHealth
	}
	for _, t := range tellers(ch.Text) {
		a.told(t)
	}
	if attacked(ch.Text) {
		a.attacked = true
	}
	if strings.Contains(ch.Text, youDied) {
		a.died = true
	}
}

// told queues an honest answer: once per person per tellEvery -- a
// socialite's answers are worth asking again for, so once per answerEvery --
// never to itself or a sibling.
func (a *Adventurer) told(t tell) {
	if strings.EqualFold(t.who, a.cfg.Name) || isSibling(t.who, a.cfg.Siblings) {
		return
	}
	every := tellEvery
	if a.cfg.Style == Socialite {
		every = answerEvery
	}
	if last, ok := a.toldAt[t.who]; ok && a.now().Sub(last) < every {
		return
	}
	a.toldAt[t.who] = a.now()
	a.pendingTells = append(a.pendingTells, t)
}

// ---- small things -------------------------------------------------------------

// pickGround chooses among the grounds its power suits and no player is
// using, at random: the first in the list every time would put every bot on
// it together.
func (a *Adventurer) pickGround(power int) *ground {
	var open []*ground
	for i := range grounds {
		g := &grounds[i]
		if power < g.minPower || power > g.maxPower {
			continue
		}
		if until, ok := a.avoiding[g.name]; ok && a.now().Before(until) {
			continue
		}
		open = append(open, g)
	}
	if len(open) == 0 {
		return nil
	}
	return open[a.rng.IntN(len(open))]
}

func (a *Adventurer) donateAt() int {
	if a.cfg.DonateAfter > 0 {
		return a.cfg.DonateAfter
	}
	return donateAfter
}

func (a *Adventurer) needsRest() bool { return a.below(restBelow) }

// below: health under pct percent of max.
func (a *Adventurer) below(pct int) bool {
	return a.maxHealth > 0 && a.health*100 < pct*a.maxHealth
}

// span is a random while in [min, max).
func (a *Adventurer) span(span [2]time.Duration) time.Duration {
	d := span[0]
	if span[1] > span[0] {
		d += time.Duration(a.rng.Int64N(int64(span[1] - span[0])))
	}
	return d
}

// pause waits a random while in span, or until ctx is done.
func (a *Adventurer) pause(ctx context.Context, span [2]time.Duration) error {
	d := a.span(span)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
