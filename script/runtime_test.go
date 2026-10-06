package script

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/testdice"
)

type line struct{ mob, text string }

type summonCall struct {
	mob, def string
	count    int
}

type harness struct {
	t        *testing.T
	rt       *Runtime
	dice     *testdice.LoadedDice
	said     []line
	summoned []summonCall
	live     map[*mobile.Instance]int  // what me.summons and the live cap read
	junk     int                       // what me:junk() answers, and sweep takes from
	swept    []int                     // each sweep's n, as the world was asked
	fled     int                       // how many times the world was asked to flee
	out      map[*mobile.Instance]bool // mobs whose fight is over, as Tick's inFight sees it
}

func newHarness(t *testing.T, scripts map[string]string) *harness {
	t.Helper()
	h := &harness{t: t, dice: testdice.New(), live: map[*mobile.Instance]int{}, out: map[*mobile.Instance]bool{}}
	programs := map[string]*Program{}
	for name, src := range scripts {
		p, err := Compile(name, src)
		require.NoError(t, err)
		programs[name] = p
	}
	rt, err := NewRuntime(programs, h.dice, Actions{
		Say: func(mob *mobile.Instance, text string) {
			h.said = append(h.said, line{mob.Name(), text})
		},
		Summon: func(mob *mobile.Instance, def *mobile.Definition, count int) int {
			h.summoned = append(h.summoned, summonCall{mob.Name(), def.Id, count})
			return count
		},
		Summons: func(mob *mobile.Instance) int { return h.live[mob] },
		Junk:    func(mob *mobile.Instance) int { return h.junk },
		Flee:    func(mob *mobile.Instance) bool { h.fled++; return true },
		Sweep: func(mob *mobile.Instance, n int) int {
			h.swept = append(h.swept, n)
			got := min(n, h.junk)
			h.junk -= got
			return got
		},
	})
	require.NoError(t, err)
	h.rt = rt
	return h
}

// summoner is a mob running script that may summon an imp
func summoner(script string) *mobile.Instance {
	m := mob("Warlock", script)
	m.Definition.Summons = []*mobile.Definition{
		mobile.NewDefinition("imp", "imp", "wrathrock", nil, "an imp", "An imp is here.",
			5, rules.WanderDefinition{}, 10, false),
	}
	return m
}

// failures collects what a harness logs
func (h *harness) failures() *[]string {
	var logged []string
	h.rt.logFailure = func(err error) { logged = append(logged, err.Error()) }
	return &logged
}

// tick is one pulse.
func (h *harness) tick() {
	h.rt.Tick(func(m *mobile.Instance) bool { return !h.out[m] })
}

func (h *harness) texts() []string {
	var out []string
	for _, l := range h.said {
		out = append(out, l.text)
	}
	return out
}

func mob(name, script string) *mobile.Instance {
	d := mobile.NewDefinition(name, name, "wrathrock", nil, name, name+" is here.",
		30, rules.WanderDefinition{}, 10, false)
	d.Script = script
	return mobile.NewInstance(d)
}

var bob = Foe{Name: "Bob", IsPlayer: true}

func TestFightStart_says(t *testing.T) {
	h := newHarness(t, map[string]string{"z/king": `
		function on_fight_start(me, foe)
			me:say("You dare, " .. foe.name .. "?")
		end`})
	king := mob("King", "z/king")

	h.rt.FightStart(king, bob)

	assert.Equal(t, []line{{"King", "You dare, Bob?"}}, h.said)
}

func TestNoScriptIsANoOp(t *testing.T) {
	h := newHarness(t, nil)
	h.rt.FightStart(mob("Rat", ""), bob)
	h.rt.FightPulse(mob("Rat", ""), bob)
	h.rt.Forget(mob("Rat", ""))
	assert.Empty(t, h.said)
}

func TestAHookNotDefinedIsANoOp(t *testing.T) {
	h := newHarness(t, map[string]string{"z/opener": `
		function on_fight_start(me, foe) me:say("hi") end`})
	h.rt.FightPulse(mob("M", "z/opener"), bob)
	assert.Empty(t, h.said)
}

func TestMeAndFoe(t *testing.T) {
	h := newHarness(t, map[string]string{"z/look": `
		function on_fight_pulse(me, foe)
			me:say(me.name .. " " .. me.health .. "/" .. me.max_health .. " vs " .. foe.name .. " " .. tostring(foe.is_player))
			me.health = 1 -- a copy: changes nothing
		end`})
	m := mob("Ghoul", "z/look")
	m.TakeMeleeDamage(5)

	h.rt.FightPulse(m, bob)

	assert.Equal(t, []string{"Ghoul 25/30 vs Bob true"}, h.texts())
	assert.Equal(t, 25, m.CurHealth)
}

func TestChanceAndPickRollTheDice(t *testing.T) {
	h := newHarness(t, map[string]string{"z/dice": `
		function on_fight_pulse(me, foe)
			if chance(50) then me:say(pick({"a", "b", "c"})) end
		end`})
	m := mob("M", "z/dice")

	h.dice.Load([]int{49, 2}) // 49 < 50: yes; index 2 is "c"
	h.rt.FightPulse(m, bob)
	h.dice.Load([]int{50}) // 50 is not < 50
	h.rt.FightPulse(m, bob)

	assert.Equal(t, []string{"c"}, h.texts())
}

func TestMemoryPersistsPerInstance(t *testing.T) {
	h := newHarness(t, map[string]string{"z/count": `
		function on_fight_pulse(me, foe)
			me.memory.n = (me.memory.n or 0) + 1
			me:say(tostring(me.memory.n))
		end`})
	one, two := mob("M", "z/count"), mob("M", "z/count")

	h.rt.FightPulse(one, bob)
	h.rt.FightPulse(one, bob)
	h.rt.FightPulse(two, bob)
	h.rt.Forget(one)
	h.rt.FightPulse(one, bob)

	assert.Equal(t, []string{"1", "2", "1", "1"}, h.texts(),
		"one counts up, two has its own, and Forget empties one's")
}

func TestProgramsDontShareGlobals(t *testing.T) {
	h := newHarness(t, map[string]string{
		"z/a": `secret = "a"
			function on_fight_start(me, foe) me:say(tostring(secret)) end`,
		"z/b": `function on_fight_start(me, foe) me:say(tostring(secret)) end`,
	})
	h.rt.FightStart(mob("A", "z/a"), bob)
	h.rt.FightStart(mob("B", "z/b"), bob)
	assert.Equal(t, []string{"a", "nil"}, h.texts())
}

func TestUnsafeLibrariesAreGone(t *testing.T) {
	h := newHarness(t, map[string]string{"z/probe": `
		function on_fight_start(me, foe)
			me:say(table.concat({
				tostring(os), tostring(io), tostring(require), tostring(load),
				tostring(loadstring), tostring(dofile), tostring(print),
				tostring(math.random), tostring(string.dump), tostring(coroutine),
				tostring(getfenv), tostring(debug), tostring(package),
			}, ","))
		end`})
	h.rt.FightStart(mob("M", "z/probe"), bob)
	require.Len(t, h.said, 1)
	assert.Equal(t, strings.Repeat("nil,", 12)+"nil", h.said[0].text)
}

// An error, a runaway loop and runaway recursion each come back, counted; the
// fourth call is never made.
func TestFailuresAreCountedAndDisable(t *testing.T) {
	for name, body := range map[string]string{
		"error":     `error("boom")`,
		"loop":      `while true do end`,
		"recursion": `local function f(n) return f(n + 1) + 1 end f(1)`,
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, map[string]string{"z/bad": `
				function on_fight_pulse(me, foe)
					me:say("tick")
					` + body + `
				end`})
			m := mob("M", "z/bad")
			for range MaxFailures + 2 {
				h.rt.FightPulse(m, bob)
			}
			assert.Len(t, h.said, MaxFailures, "called until the budget ran out, then never")
		})
	}
}

// One bad script doesn't switch off another.
func TestDisablingIsPerProgram(t *testing.T) {
	h := newHarness(t, map[string]string{
		"z/bad":  `function on_fight_pulse(me, foe) error("boom") end`,
		"z/good": `function on_fight_pulse(me, foe) me:say("fine") end`,
	})
	for range MaxFailures {
		h.rt.FightPulse(mob("B", "z/bad"), bob)
	}
	h.rt.FightPulse(mob("G", "z/good"), bob)
	assert.Equal(t, []string{"fine"}, h.texts())
}

// Review focus 1: a dot instead of a colon is an error that says so.
func TestSayWithADotFails(t *testing.T) {
	h := newHarness(t, map[string]string{"z/dot": `
		function on_fight_start(me, foe) me.say("hi") end`})
	var logged []string
	h.rt.logFailure = func(err error) { logged = append(logged, err.Error()) }

	h.rt.FightStart(mob("M", "z/dot"), bob)

	assert.Empty(t, h.said)
	require.Len(t, logged, 1)
	assert.Contains(t, logged[0], "colon")
}

// Review focus 2: a me kept from an earlier call can't be spoken through.
func TestStashedMeCantSpeakLater(t *testing.T) {
	h := newHarness(t, map[string]string{"z/stash": `
		function on_fight_start(me, foe) me.memory.old = me end
		function on_fight_pulse(me, foe) me.memory.old:say("ghost") end`})
	m := mob("M", "z/stash")

	h.rt.FightStart(m, bob)
	h.rt.FightPulse(m, bob)

	assert.Empty(t, h.said)
}

// Review focus 3: an empty pick is an error, not a panic or a nil said.
func TestPickEmptyFails(t *testing.T) {
	h := newHarness(t, map[string]string{"z/empty": `
		function on_fight_start(me, foe) me:say(pick({})) end`})
	var logged []string
	h.rt.logFailure = func(err error) { logged = append(logged, err.Error()) }

	h.rt.FightStart(mob("M", "z/empty"), bob)

	assert.Empty(t, h.said)
	require.Len(t, logged, 1)
	assert.Contains(t, logged[0], "empty")
}

// Review finding 1: a runaway say is a failure, not a flood. Every say is a
// Send to everyone in the room, and a full send queue hangs a player up.
func TestSayFloodIsCappedAndFails(t *testing.T) {
	h := newHarness(t, map[string]string{"z/flood": `
		function on_fight_start(me, foe) while true do me:say("hah") end end`})
	var logged []string
	h.rt.logFailure = func(err error) { logged = append(logged, err.Error()) }

	h.rt.FightStart(mob("M", "z/flood"), bob)

	assert.Len(t, h.said, MaxSaysPerCall)
	require.Len(t, logged, 1)
	assert.Contains(t, logged[0], "says")
}

func TestSayTooLongFails(t *testing.T) {
	h := newHarness(t, map[string]string{"z/long": `
		function on_fight_start(me, foe) me:say(string.rep("a", 301)) end`})
	var logged []string
	h.rt.logFailure = func(err error) { logged = append(logged, err.Error()) }

	h.rt.FightStart(mob("M", "z/long"), bob)

	assert.Empty(t, h.said)
	require.Len(t, logged, 1)
	assert.Contains(t, logged[0], "longer than")
}

// Review finding 2: Go builtins aren't interrupted by the deadline, so the
// ones whose work a script controls are gone or capped before they start.
func TestPatternFunctionsAreGone(t *testing.T) {
	h := newHarness(t, map[string]string{"z/probe": `
		function on_fight_start(me, foe)
			me:say(tostring(string.find) .. tostring(string.gsub) .. tostring(string.match) ..
				tostring(string.gmatch) .. tostring(string.gfind) .. tostring(("x").find))
		end`})
	h.rt.FightStart(mob("M", "z/probe"), bob)
	assert.Equal(t, []string{strings.Repeat("nil", 6)}, h.texts())
}

// Refused before the work, not noticed by the deadline afterwards: an
// uncapped rep of 2^30 allocates a gigabyte and takes a quarter of a second,
// with the whole world waiting on it.
func TestRepIsCapped(t *testing.T) {
	h := newHarness(t, map[string]string{"z/big": `
		function on_fight_start(me, foe) me:say(string.rep("ab", 3)) end
		function on_fight_pulse(me, foe) local s = string.rep("x", 2^30) end`})
	var logged []string
	h.rt.logFailure = func(err error) { logged = append(logged, err.Error()) }
	m := mob("M", "z/big")

	h.rt.FightStart(m, bob)
	start := time.Now()
	h.rt.FightPulse(m, bob)

	assert.Less(t, time.Since(start), 50*time.Millisecond)
	assert.Equal(t, []string{"ababab"}, h.texts(), "a small rep still works")
	require.Len(t, logged, 1)
	assert.Contains(t, logged[0], "longer than")
}

func TestFormatWidthIsCapped(t *testing.T) {
	h := newHarness(t, map[string]string{"z/fmt": `
		function on_fight_start(me, foe) me:say(string.format("[%5s]", "hi")) end
		function on_fight_pulse(me, foe) local s = string.format("%999999d", 1) end`})
	var logged []string
	h.rt.logFailure = func(err error) { logged = append(logged, err.Error()) }
	m := mob("M", "z/fmt")

	h.rt.FightStart(m, bob)
	h.rt.FightPulse(m, bob)

	assert.Equal(t, []string{"[   hi]"}, h.texts())
	require.Len(t, logged, 1)
	assert.Contains(t, logged[0], "format")
}

// Review finding 3: each program has its own copies of the libraries, so
// one poisoning them leaves the others alone -- at load or in a hook.
func TestProgramsCantPoisonEachOther(t *testing.T) {
	h := newHarness(t, map[string]string{
		"z/a": `
			string.upper = function() return "pwned" end
			function on_fight_start(me, foe)
				pcall(function() string.format = function() return "pwned" end end)
				pcall(function() _G.pick = nil end)
				pcall(function() getmetatable("").__index = {} end)
				me:say("done")
			end`,
		"z/b": `function on_fight_start(me, foe)
				me:say(string.format("%s", pick({"ok"})) .. string.upper("x") .. ("y"):upper())
			end`,
	})
	h.dice.Load([]int{0}) // B's pick
	h.rt.FightStart(mob("A", "z/a"), bob)
	h.rt.FightStart(mob("B", "z/b"), bob)
	assert.Equal(t, []string{"done", "okXY"}, h.texts())
}

func TestSummon(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me:say(tostring(me:summon("imp", 2))) end`})

	h.rt.FightStart(summoner("z/w"), bob)

	assert.Equal(t, []summonCall{{"Warlock", "imp", 2}}, h.summoned)
	assert.Equal(t, []string{"2"}, h.texts(), "it answers how many came")
}

// named the way mobs.json may name it: "zone/id"
func TestSummonByZoneAndId(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me:summon("wrathrock/imp", 1) end`})
	h.rt.FightStart(summoner("z/w"), bob)
	assert.Equal(t, []summonCall{{"Warlock", "imp", 1}}, h.summoned)
}

func TestSummonUndeclaredFails(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me:summon("dragon", 1) end`})
	logged := h.failures()

	h.rt.FightStart(summoner("z/w"), bob)

	assert.Empty(t, h.summoned)
	require.Len(t, *logged, 1)
	assert.Contains(t, (*logged)[0], "may not summon")
}

func TestSummonCountUnderOneFails(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me:summon("imp", 0) end`})
	logged := h.failures()

	h.rt.FightStart(summoner("z/w"), bob)

	assert.Empty(t, h.summoned)
	require.Len(t, *logged, 1)
}

// Review focus 4: a loop gets MaxSummonsPerCall across the whole call, and
// no error -- the cap is a limit, not a mistake
func TestSummonPerCallCap(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe)
			for i = 1, 10 do me:summon("imp", 3) end
		end`})
	logged := h.failures()

	h.rt.FightStart(summoner("z/w"), bob)

	assert.Equal(t, []summonCall{{"Warlock", "imp", 3}, {"Warlock", "imp", 1}}, h.summoned)
	assert.Empty(t, *logged)
}

// and MaxLiveSummons alive at once: three standing leaves room for one
func TestSummonLiveCap(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me:say(tostring(me:summon("imp", 2))) end`})
	m := summoner("z/w")
	h.live[m] = 3

	h.rt.FightStart(m, bob)

	assert.Equal(t, []summonCall{{"Warlock", "imp", 1}}, h.summoned)
	assert.Equal(t, []string{"1"}, h.texts())
}

func TestSummonsReadsTheLiveCount(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me:say(tostring(me.summons)) end`})
	m := summoner("z/w")
	h.live[m] = 2

	h.rt.FightStart(m, bob)

	assert.Equal(t, []string{"2"}, h.texts())
}

func TestStashedMeCantSummonLater(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me.memory.old = me end
		function on_fight_pulse(me, foe) me.memory.old:summon("imp", 1) end`})
	m := summoner("z/w")

	h.rt.FightStart(m, bob)
	h.rt.FightPulse(m, bob)

	assert.Empty(t, h.summoned)
}

func TestSummonWithADotFails(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me.summon("imp", 1) end`})
	logged := h.failures()

	h.rt.FightStart(summoner("z/w"), bob)

	require.Len(t, *logged, 1)
	assert.Contains(t, (*logged)[0], "colon")
}

func TestWaitDefersTheRest(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me:say("one") wait(2) me:say("two") end`})
	m := mob("M", "z/w")

	h.rt.FightStart(m, bob)
	assert.Equal(t, []string{"one"}, h.texts())
	h.tick()
	assert.Equal(t, []string{"one"}, h.texts(), "one pulse in: still waiting")
	h.tick()
	assert.Equal(t, []string{"one", "two"}, h.texts())
	h.tick()
	assert.Equal(t, []string{"one", "two"}, h.texts(), "and done")
}

func TestWaitRefreshesMe(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe)
			wait(1)
			me:say(me.health .. " " .. me.summons)
		end`})
	m := summoner("z/w")
	h.rt.FightStart(m, bob)

	m.CurHealth = 7
	h.live[m] = 1
	h.tick()

	assert.Equal(t, []string{"7 1"}, h.texts())
}

func TestSummonAfterAWait(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) wait(1) me:summon("imp", 2) end`})
	h.rt.FightStart(summoner("z/w"), bob)
	assert.Empty(t, h.summoned)

	h.tick()

	assert.Equal(t, []summonCall{{"Warlock", "imp", 2}}, h.summoned)
}

// one run is one call: the say cap counts across the wait
func TestCapsSpanAWait(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe)
			me:say("a") me:say("b") wait(1) me:say("c")
		end`})
	logged := h.failures()

	h.rt.FightStart(mob("M", "z/w"), bob)
	h.tick()

	assert.Equal(t, []string{"a", "b"}, h.texts())
	require.Len(t, *logged, 1)
	assert.Contains(t, (*logged)[0], "says")
}

// its own me still works after a wait (above); one kept from another call doesn't
func TestStashedMeCantActAfterAWait(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me.memory.old = me end
		function on_fight_pulse(me, foe) wait(1) me.memory.old:say("ghost") end`})
	m := mob("M", "z/w")
	logged := h.failures()

	h.rt.FightStart(m, bob)
	h.rt.FightPulse(m, bob)
	h.tick()

	assert.Empty(t, h.said)
	require.Len(t, *logged, 1)
	assert.Contains(t, (*logged)[0], "earlier call")
}

// Review focus 5 among them: 1.5 is refused, not cut to 1
func TestWaitBadArguments(t *testing.T) {
	for _, arg := range []string{"0", "-1", "11", "1.5", `"x"`} {
		t.Run(arg, func(t *testing.T) {
			h := newHarness(t, map[string]string{"z/w": `
				function on_fight_start(me, foe) wait(` + arg + `) me:say("after") end`})
			logged := h.failures()

			h.rt.FightStart(mob("M", "z/w"), bob)
			for range 12 {
				h.tick()
			}

			assert.Empty(t, h.said)
			assert.Len(t, *logged, 1)
		})
	}
}

func TestWaitAtTopLevelFailsCompile(t *testing.T) {
	_, err := Compile("z/top", `wait(1)`)
	assert.ErrorContains(t, err, "only inside a hook")
}

func TestWaitPerCallCap(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe)
			wait(10) wait(10) wait(10) wait(1) me:say("never")
		end`})
	logged := h.failures()

	h.rt.FightStart(mob("M", "z/w"), bob)
	for range 32 {
		h.tick()
	}

	assert.Empty(t, h.said)
	require.Len(t, *logged, 1)
	assert.Contains(t, (*logged)[0], "seconds")
}

// in the order they began waiting, whatever the map order of anything else
func TestWaitersResumeInOrder(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) wait(1) me:say(me.name) end`})
	a, b, c := mob("A", "z/w"), mob("B", "z/w"), mob("C", "z/w")

	h.rt.FightStart(b, bob)
	h.rt.FightStart(c, bob)
	h.rt.FightStart(a, bob)
	h.tick()

	assert.Equal(t, []string{"B", "C", "A"}, h.texts())
}

// an error after a wait is a strike like any other
func TestErrorAfterAWaitIsAStrike(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me:say("tick") wait(1) error("boom") end`})
	m := mob("M", "z/w")

	for range MaxFailures + 2 {
		h.rt.FightStart(m, bob)
		h.tick()
	}

	assert.Len(t, h.said, MaxFailures, "switched off after the third")
}

// Review focus 3: a script author's reflex. gopher-lua's pcall treats a
// yield as a return, so wait would silently not wait; it refuses instead,
// and pcall hands the script the error.
func TestWaitInsidePcall(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe)
			local ok, err = pcall(wait, 1)
			me:say(tostring(ok))
			me:say(err)
		end`})

	h.rt.FightStart(mob("M", "z/w"), bob)

	require.Len(t, h.said, 2)
	assert.Equal(t, "false", h.said[0].text)
	assert.Contains(t, h.said[1].text, "pcall")
}

// one thing at a time: while a hook waits, the mob's hooks don't fire --
// those calls are skipped, not queued
func TestNoHooksWhileWaiting(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me:say("start") wait(2) me:say("end") end
		function on_fight_pulse(me, foe) me:say("pulse") end`})
	m := mob("M", "z/w")

	h.rt.FightStart(m, bob)
	h.rt.FightPulse(m, bob)
	h.rt.FightStart(m, bob)
	h.tick()
	h.tick()
	h.rt.FightPulse(m, bob)

	assert.Equal(t, []string{"start", "end", "pulse"}, h.texts())
}

// the fight is over when the wait comes due: dropped, not failed, and the
// mob's hooks fire again
func TestFightOverDropsTheWait(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me:say("one") wait(1) me:say("two") end
		function on_fight_pulse(me, foe) me:say("again") end`})
	m := mob("M", "z/w")
	logged := h.failures()

	h.rt.FightStart(m, bob)
	h.out[m] = true
	h.tick()
	h.out[m] = false
	h.rt.FightPulse(m, bob)

	assert.Equal(t, []string{"one", "again"}, h.texts())
	assert.Empty(t, *logged)
}

// the mob leaves the world: its wait goes with its memory
func TestForgetDropsTheWait(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me:say("one") wait(1) me:say("two") end`})
	m := mob("M", "z/w")

	h.rt.FightStart(m, bob)
	h.rt.Forget(m)
	h.tick()

	assert.Equal(t, []string{"one"}, h.texts())
}

// Review focus 4: another mob's errors switch the program off, and the
// waiting run of it never carries on
func TestDisabledProgramDropsItsWaits(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) wait(1) me:say("after") end
		function on_fight_pulse(me, foe) error("boom") end`})
	waiter, breaker := mob("W", "z/w"), mob("B", "z/w")

	h.rt.FightStart(waiter, bob)
	for range MaxFailures {
		h.rt.FightPulse(breaker, bob)
	}
	h.tick()

	assert.Empty(t, h.said)
}

// xpcall is protected the same way
func TestWaitInsideXpcall(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe)
			local ok = xpcall(function() wait(1) end, function(e) return e end)
			me:say(tostring(ok))
		end`})

	h.rt.FightStart(mob("M", "z/w"), bob)

	assert.Equal(t, []string{"false"}, h.texts())
}

// once a pcall has returned -- normally or with an error caught -- wait
// waits again: the guard is only for the inside of one
func TestWaitAfterAPcall(t *testing.T) {
	for name, inner := range map[string]string{
		"returned": `function() end`,
		"errored":  `function() error("boom") end`,
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, map[string]string{"z/w": `
				function on_fight_start(me, foe)
					pcall(` + inner + `)
					wait(1)
					me:say("after")
				end`})

			h.rt.FightStart(mob("M", "z/w"), bob)
			assert.Empty(t, h.said, "still waiting")
			h.tick()

			assert.Equal(t, []string{"after"}, h.texts())
		})
	}
}

// A hook that summons a scripted mob starts that mob's opener from inside
// itself. Lua can't run a second hook in the middle of the first -- the
// first would come back to find its call gone -- so the summon's opener runs
// once the summoner's hook has finished or paused.
func TestSummoningAScriptedMob(t *testing.T) {
	h := newHarness(t, map[string]string{
		"z/boss": `function on_fight_start(me, foe) me:summon("imp", 1) me:say("after") end`,
		"z/imp":  `function on_fight_start(me, foe) me:say("imp opener") end`,
	})
	boss := summoner("z/boss")
	boss.Definition.Summons[0].Script = "z/imp"
	h.rt.actions.Summon = func(_ *mobile.Instance, def *mobile.Definition, n int) int {
		h.rt.FightStart(mobile.NewInstance(def), bob) // what World.summon's startFight does
		return n
	}
	logged := h.failures()

	h.rt.FightStart(boss, bob)

	assert.Equal(t, []string{"after", "imp opener"}, h.texts())
	assert.Empty(t, *logged)
}

// on_hear gets who spoke, what they said, and its words to look up
func TestHear(t *testing.T) {
	h := newHarness(t, map[string]string{"z/witch": `
		function on_hear(me, speaker, said)
			if said.words.heal then
				me:say(speaker.name .. " wants healing: " .. said.text)
			end
		end`})
	witch := mob("Witch", "z/witch")

	h.rt.Hear(witch, bob, "Can you HEAL me, please?")
	h.rt.Hear(witch, bob, "nice weather")
	h.rt.Hear(witch, bob, "healer?")

	assert.Equal(t, []string{"Bob wants healing: Can you HEAL me, please?"}, h.texts(), "a word, not part of one")
}

func TestHear_wordsKeepApostrophesInside(t *testing.T) {
	h := newHarness(t, map[string]string{"z/witch": `
		function on_hear(me, speaker, said)
			if said.words["don't"] and said.words["go"] then me:say("ok") end
		end`})
	h.rt.Hear(mob("Witch", "z/witch"), bob, "'Don't' go!")
	assert.Equal(t, []string{"ok"}, h.texts())
}

// a wait in on_hear isn't about a fight, so no fight doesn't end it
func TestHear_waitNeedsNoFight(t *testing.T) {
	h := newHarness(t, map[string]string{"z/witch": `
		function on_hear(me, speaker, said)
			wait(1)
			me:say("Hm?")
		end`})
	witch := mob("Witch", "z/witch")
	h.out[witch] = true // not fighting

	h.rt.Hear(witch, bob, "hello")
	assert.Empty(t, h.said)
	h.tick()
	assert.Equal(t, []string{"Hm?"}, h.texts())
}

func TestHear_isAHook(t *testing.T) {
	_, err := Compile("z/witch", `function on_hear(me, speaker, said) end`)
	assert.NoError(t, err)
}

// the real hedge-witch: asked about healing, a beat, then her two lines
func TestHedgeWitch(t *testing.T) {
	src, err := os.ReadFile("../content/world/hollowfield/scripts/hedge_witch.lua")
	require.NoError(t, err)
	h := newHarness(t, map[string]string{"hollowfield/hedge_witch": string(src)})
	witch := mob("hedge-witch", "hollowfield/hedge_witch")
	h.out[witch] = true

	h.rt.Hear(witch, bob, "lovely herbs")
	h.rt.Hear(witch, bob, "I'm hurt, can you help?")
	assert.Empty(t, h.said)
	h.tick()
	require.Len(t, h.said, 2)
	assert.Contains(t, h.said[1].text, "Bob")
}

// on_arrive gets me alone; junk and sweep reach the world, sweep cut to the cap
func TestArrive_junkAndSweep(t *testing.T) {
	h := newHarness(t, map[string]string{"z/janitor": `
		function on_arrive(me)
			local n = me:junk()
			local a = me:sweep(4)
			local b = me:sweep(4)
			me:say(n .. " " .. a .. " " .. b)
		end`})
	h.junk = 9

	h.rt.Arrive(mob("Janitor", "z/janitor"))

	assert.Equal(t, []string{"9 4 1"}, h.texts(), "five a call, across both sweeps")
	assert.Equal(t, []int{4, 1}, h.swept)
}

func TestArrive_sweepCountMustBePositive(t *testing.T) {
	h := newHarness(t, map[string]string{"z/janitor": `
		function on_arrive(me) me:sweep(0) end`})
	logged := h.failures()
	h.rt.Arrive(mob("Janitor", "z/janitor"))
	require.Len(t, *logged, 1)
	assert.Contains(t, (*logged)[0], "a count of 0")
}

// a wait in on_arrive isn't about a fight either
func TestArrive_waitNeedsNoFight(t *testing.T) {
	h := newHarness(t, map[string]string{"z/janitor": `
		function on_arrive(me) wait(2); me:say("there") end`})
	j := mob("Janitor", "z/janitor")
	h.out[j] = true
	h.rt.Arrive(j)
	h.tick()
	assert.Empty(t, h.said)
	h.tick()
	assert.Equal(t, []string{"there"}, h.texts())
}

// the real janitor: junk, a beat, a sweep of three
func TestJanitor(t *testing.T) {
	src, err := os.ReadFile("../content/world/wrathrock/scripts/janitor.lua")
	require.NoError(t, err)
	h := newHarness(t, map[string]string{"wrathrock/janitor": string(src)})
	j := mob("janitor", "wrathrock/janitor")
	h.out[j] = true

	h.rt.Arrive(j) // nothing to sweep
	h.tick()
	h.tick()
	assert.Empty(t, h.swept)

	h.junk = 7
	h.rt.Arrive(j)
	assert.Empty(t, h.swept, "a beat first")
	h.tick()
	h.tick()
	assert.Equal(t, []int{3}, h.swept)
	assert.Equal(t, 4, h.junk)
}

// me:flee() reaches the world once a call, however often it's asked
func TestFlee_onceACall(t *testing.T) {
	h := newHarness(t, map[string]string{"z/coward": `
		function on_fight_pulse(me, foe)
			local a, b = me:flee(), me:flee()
			me:say(tostring(a) .. " " .. tostring(b))
		end`})
	h.rt.FightPulse(mob("Coward", "z/coward"), bob)
	assert.Equal(t, 1, h.fled)
	assert.Equal(t, []string{"true false"}, h.texts())
}

// the real bandit: only when losing, only once, and only on the coin flip
func TestBandit(t *testing.T) {
	src, err := os.ReadFile("../content/world/hollowfield/scripts/bandit.lua")
	require.NoError(t, err)
	h := newHarness(t, map[string]string{"hollowfield/bandit": string(src)})
	b := mob("bandit", "hollowfield/bandit")

	h.rt.FightPulse(b, bob)
	assert.Zero(t, h.fled, "healthy: it fights on")

	b.CurHealth = b.Definition.MaxHealth / 5
	h.dice.Load([]int{0, 0}) // chance(50) comes up; pick the first plea
	h.rt.FightPulse(b, bob)
	assert.Equal(t, 1, h.fled)
	assert.Len(t, h.said, 1)

	h.rt.FightPulse(b, bob)
	assert.Equal(t, 1, h.fled, "once a life")
}

// the real shopkeeper: a topic she knows, a hello, and quiet otherwise
func TestShopkeeper(t *testing.T) {
	src, err := os.ReadFile("../content/world/wrathrock/scripts/shopkeeper.lua")
	require.NoError(t, err)
	h := newHarness(t, map[string]string{"wrathrock/shopkeeper": string(src)})
	keeper := mob("shopkeeper", "wrathrock/shopkeeper")

	h.rt.Hear(keeper, bob, "what would you pay for this pelt?")
	h.rt.Hear(keeper, bob, "Hello!")
	h.rt.Hear(keeper, bob, "nice weather")
	require.Len(t, h.said, 2)
	assert.Contains(t, h.said[0].text, "'value'")
	assert.Contains(t, h.said[1].text, "Welcome in, Bob")
}
