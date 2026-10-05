package script

import (
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
	live     map[*mobile.Instance]int // what me.summons and the live cap read
}

func newHarness(t *testing.T, scripts map[string]string) *harness {
	t.Helper()
	h := &harness{t: t, dice: testdice.New(), live: map[*mobile.Instance]int{}}
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
