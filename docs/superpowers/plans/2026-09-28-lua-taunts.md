# Lua Taunts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The first Lua in watchmud: a mob that names a script says something when a fight starts and now and then during it, and a broken script is logged and switched off rather than hurting the server.

**Architecture:** A new `script/` package owns gopher-lua: `Compile` validates a file in a throwaway sandbox, `Runtime` holds the one real Lua state and calls hooks under a time limit and a failure budget. The loader compiles every script a mob names into `loader.Content.Scripts`; `world` builds the `Runtime` in `world.New` and fires `on_fight_start` from a new `startFight` (used by `kill` and aggro) and `on_fight_pulse` from `DoViolence`.

**Tech Stack:** Go 1.27, `github.com/yuin/gopher-lua` v1.1.2, testify, `testdice.LoadedDice`.

**Spec:** `docs/superpowers/specs/2026-09-28-lua-taunts-design.md`

## Global Constraints

- One Lua state for the running world, on the world goroutine, no locks. `Compile` may use a throwaway state of its own for validation.
- Every hook call and every top level runs under `script.CallTimeout` = **10ms**.
- **3** failures (error, timeout, panic) and a program is disabled until restart, with one warning.
- Only base, string, table, math are opened. Removed: `dofile`, `loadfile`, `load`, `loadstring`, `require`, `module`, `getfenv`, `setfenv`, `print`, `collectgarbage`, `newproxy`, `_printregs`, `math.random`, `math.randomseed`, `string.dump`. No `os`, `io`, `package`, `debug`, `coroutine`, `channel`.
- `chance(pct)` is `roller.IntN(100) < pct`; `pick(list)` is `list[roller.IntN(#list) + 1]`. Both only inside a hook.
- `script/` never imports `world`. `mobile` never imports `script` (`Definition.Script` is a string).
- Hooks: exactly `on_fight_start(me, foe)` and `on_fight_pulse(me, foe)`. Any other global starting `on_`, or a hook name bound to a non-function, fails `Compile`.
- A script name in mobs.json: bare = the mob's zone, `"zone/name"` = any zone. File: `world/<zone>/scripts/<name>.lua`. Canonical ref stored everywhere: `"<zone>/<name>"`.
- Logging in new code is zerolog (`github.com/rs/zerolog/log`).
- Every commit: `make check` green first, commit to `master`, message ends with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **`me.say("hi")` with a dot instead of a colon** — the commonest Lua slip. Expect a clear error naming the colon, counted like any failure, not a silent no-op or a confusing "bad argument #2". Test in Task 2.
2. **A script that stashes `me` in `me.memory` and calls `:say` on it from a later hook** — expect an error; a mob must never speak through another call's `me`. Test in Task 2.
3. **`pick({})`** — expect an error (counted), not an index-out-of-range panic or a nil said to the room. Test in Task 2.
4. **A top level that rolls dice or speaks** (`chance(10)` at file scope) — expect `Compile` to fail with "only inside a hook", so it can't consume test dice twice or talk at startup. Test in Task 1.
5. **`on_fight_start = "hello"`** (a known hook name bound to a non-function) — expect `Compile` to fail, rather than the hook silently never running. Test in Task 1.

---

## File Structure

- Create `script/sandbox.go` — the Lua state with safe libraries, `chance`/`pick`, `call` (deadline + protection + recover), `load` (per-program environment table). Private to the package.
- Create `script/program.go` — `Program`, `Compile`, hook-name checking.
- Create `script/runtime.go` — `Runtime`, `Foe`, `Say`, `Roller`, `NewRuntime`, `FightStart`, `FightPulse`, `Forget`, the failure budget, building `me`/`foe`.
- Create `script/program_test.go`, `script/runtime_test.go`.
- Modify `mobile/definition.go` — `Script string` field.
- Modify `loader/mobfile.go` (`Script` on `mobEntry`), `loader/content.go` (`Scripts` on `Content`, call `mobScript`), create `loader/script.go` (`mobScript`) and `loader/script_test.go`.
- Create `world/scripts.go` — `startFight`, `foeOf`, `mobSays`. Modify `world/world.go` (runtime field, `New`, `RemoveMobile`), `world/h_kill.go`, `world/mobile_activity.go`, `world/violence.go`. Create `world/scripts_test.go`.
- Create `testcontent/world/wrathrock/scripts/heckler.lua`; modify `testcontent/world/wrathrock/mobs.json`, `instructions.json`.
- Create `telnet/render_script_test.go`; modify `telnet/render_test.go:304`.
- Create `content/world/barrow/scripts/barrow_king.lua`; modify `content/world/barrow/mobs.json`.
- Modify `CLAUDE.md`, `ROADMAP.md`.

---

### Task 1: `script.Compile` and the sandbox

**Files:**
- Create: `script/sandbox.go`, `script/program.go`, `script/program_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Produces:
  - `type Program struct` (unexported fields `name string`, `proto *lua.FunctionProto`), `func (p *Program) Name() string`
  - `func Compile(name, src string) (*Program, error)`
  - `const CallTimeout = 10 * time.Millisecond`
  - `type Roller interface { IntN(n int) (int, error) }`
  - unexported, used by Task 2: `type hookCall struct { roller Roller; say func(text string) }`, `type sandbox struct { L *lua.LState; current *hookCall }`, `func newSandbox() *sandbox`, `func (s *sandbox) call(fn *lua.LFunction, h *hookCall, args ...lua.LValue) error`, `func (s *sandbox) load(p *Program) (*lua.LTable, error)`, `var hooks map[string]bool`

- [ ] **Step 1: Add the dependency**

Run: `go get github.com/yuin/gopher-lua@v1.1.2`
Expected: go.mod gains `github.com/yuin/gopher-lua v1.1.2` (it will be marked `// indirect` until code imports it; `go mod tidy` in Step 5 fixes that).

- [ ] **Step 2: Write the failing tests**

`script/program_test.go`:

```go
package script

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompile_hooks(t *testing.T) {
	p, err := Compile("wrathrock/ok", `
		local lines = { "hi" }
		function on_fight_start(me, foe) end
		function on_fight_pulse(me, foe) end
	`)
	require.NoError(t, err)
	assert.Equal(t, "wrathrock/ok", p.Name())
}

func TestCompile_noHooksIsFine(t *testing.T) {
	_, err := Compile("wrathrock/quiet", `local x = 1`)
	assert.NoError(t, err)
}

func TestCompile_syntaxError(t *testing.T) {
	_, err := Compile("wrathrock/broken", `function on_fight_start(me, foe`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wrathrock/broken")
}

// A typo'd hook would otherwise never run, silently.
func TestCompile_unknownHook(t *testing.T) {
	_, err := Compile("wrathrock/typo", `function on_fightstart(me, foe) end`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "on_fightstart")
	assert.Contains(t, err.Error(), "on_fight_start") // the error lists what is known
}

// Review focus 5: a known name that isn't a function would never run either.
func TestCompile_hookThatIsNotAFunction(t *testing.T) {
	_, err := Compile("wrathrock/str", `on_fight_start = "hello"`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a function")
}

// A local named on_x isn't a global, so it isn't a hook.
func TestCompile_localOnNameIsNotAHook(t *testing.T) {
	_, err := Compile("wrathrock/local", `local function on_helper() end`)
	assert.NoError(t, err)
}

// The top level runs under the deadline too: a builder's infinite loop fails
// the load instead of hanging startup.
func TestCompile_topLevelThatNeverEnds(t *testing.T) {
	_, err := Compile("wrathrock/spin", `while true do end`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadline")
}

func TestCompile_topLevelError(t *testing.T) {
	_, err := Compile("wrathrock/boom", `error("boom")`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

// Review focus 4: dice and speech belong to hooks. A top level that rolls
// would consume dice at load; one that speaks would talk to no room.
func TestCompile_topLevelCantRoll(t *testing.T) {
	_, err := Compile("wrathrock/early", `local lucky = chance(10)`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only inside a hook")

	_, err = Compile("wrathrock/early2", `local x = pick({1, 2})`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only inside a hook")
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./script/`
Expected: FAIL to build — `undefined: Compile`.

- [ ] **Step 4: Write the implementation**

`script/sandbox.go`:

```go
// Package script runs the Lua a builder attaches to a mob. Go is the engine:
// a script decides *when*, from actions the engine already has, and never how
// the math works. See CLAUDE.md, "Scripts (Lua)".
package script

import (
	"context"
	"fmt"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// CallTimeout is how long one hook call -- or a program's top level, at load
// -- may run before it is stopped. Huge for a taunt; a script that needs
// longer is doing something a script shouldn't.
const CallTimeout = 10 * time.Millisecond

const (
	// callStackSize caps Lua call depth, so runaway recursion is a Lua error
	// rather than an out-of-memory crash.
	callStackSize = 128
	// registrySize and registryMaxSize cap the value stack the same way.
	registrySize    = 4 * 1024
	registryMaxSize = 64 * 1024
)

// unsafeGlobals are removed once the base library is open: everything that
// loads code from a string or a file (and so gets round Compile's checks),
// reaches another program's environment, or writes to the server's stdout.
var unsafeGlobals = []string{
	"dofile", "loadfile", "load", "loadstring", "require", "module",
	"getfenv", "setfenv", "print", "collectgarbage", "newproxy", "_printregs",
}

// Roller is the dice chance and pick roll: w.roller in the game, loaded dice
// in a test. rules.Roller satisfies it.
type Roller interface {
	IntN(n int) (int, error)
}

// hookCall is what the helpers reach while a hook is running. It is nil
// outside one, which is how a top level that tries to roll or speak is
// refused.
type hookCall struct {
	roller Roller
	say    func(text string)
}

// sandbox is one Lua state with only the safe libraries open, plus chance and
// pick.
type sandbox struct {
	L       *lua.LState
	current *hookCall
}

func newSandbox() *sandbox {
	s := &sandbox{L: lua.NewState(lua.Options{
		SkipOpenLibs:    true,
		CallStackSize:   callStackSize,
		RegistrySize:    registrySize,
		RegistryMaxSize: registryMaxSize,
	})}
	for _, lib := range []struct {
		name string
		open lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
		s.L.Push(s.L.NewFunction(lib.open))
		s.L.Push(lua.LString(lib.name))
		s.L.Call(1, 0)
	}

	g := s.L.G.Global
	for _, name := range unsafeGlobals {
		g.RawSetString(name, lua.LNil)
	}
	// Every roll goes through the roller, so a test can load the dice.
	mathLib := g.RawGetString("math").(*lua.LTable)
	mathLib.RawSetString("random", lua.LNil)
	mathLib.RawSetString("randomseed", lua.LNil)
	// The string table is also strings' metatable, so this covers ("x"):dump.
	g.RawGetString("string").(*lua.LTable).RawSetString("dump", lua.LNil)

	g.RawSetString("chance", s.L.NewFunction(s.chance))
	g.RawSetString("pick", s.L.NewFunction(s.pick))
	return s
}

// running is the hook in progress, or a Lua error for a helper called
// outside one.
func (s *sandbox) running(L *lua.LState, helper string) *hookCall {
	if s.current == nil {
		L.RaiseError("%s: only inside a hook", helper)
	}
	return s.current
}

// chance(pct) is true pct percent of the time: a d100 under pct, the same
// roll loot uses.
func (s *sandbox) chance(L *lua.LState) int {
	pct := L.CheckInt(1)
	h := s.running(L, "chance")
	n, err := h.roller.IntN(100)
	if err != nil {
		L.RaiseError("chance: %v", err)
	}
	L.Push(lua.LBool(n < pct))
	return 1
}

// pick(list) is one element of a sequence.
func (s *sandbox) pick(L *lua.LState) int {
	list := L.CheckTable(1)
	h := s.running(L, "pick")
	size := list.Len()
	if size == 0 {
		L.RaiseError("pick: the list is empty")
	}
	i, err := h.roller.IntN(size)
	if err != nil {
		L.RaiseError("pick: %v", err)
	}
	L.Push(list.RawGetInt(i + 1))
	return 1
}

// call runs fn protected, under CallTimeout, with h as the hook in progress
// (nil for a top level). An error, a timeout, a stack overflow or a panic
// inside gopher-lua all come back as an error; none of them escape.
func (s *sandbox) call(fn *lua.LFunction, h *hookCall, args ...lua.LValue) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), CallTimeout)
	defer cancel()
	s.L.SetContext(ctx)
	s.current = h
	defer func() {
		s.current = nil
		s.L.RemoveContext()
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return s.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, args...)
}

// load runs p's top level in a fresh environment table and returns that
// table: the program's own globals, which is where its hooks end up. Reads
// of anything it didn't define fall through to the shared libraries; writes
// stay in the table, so two programs' globals never meet.
func (s *sandbox) load(p *Program) (*lua.LTable, error) {
	env := s.L.NewTable()
	meta := s.L.NewTable()
	meta.RawSetString("__index", s.L.G.Global)
	s.L.SetMetatable(env, meta)

	fn := s.L.NewFunctionFromProto(p.proto)
	fn.Env = env // functions the script defines inherit it
	if err := s.call(fn, nil); err != nil {
		return nil, fmt.Errorf("script %s: %w", p.name, err)
	}
	return env, nil
}
```

`script/program.go`:

```go
package script

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"
)

// hooks is every hook the world calls, and so every global name starting
// "on_" a script may define.
var hooks = map[string]bool{
	"on_fight_start": true,
	"on_fight_pulse": true,
}

// Program is one compiled script, named by its canonical "zone/name". It
// holds no Lua state: the Runtime loads it into the world's.
type Program struct {
	name  string
	proto *lua.FunctionProto
}

func (p *Program) Name() string { return p.name }

// Compile parses src and proves it loads: the top level runs in a throwaway
// sandbox, under the same limits a hook gets, and may define only the hooks
// the world calls. Anything wrong is an error naming the script, for the
// loader to fail startup with.
func Compile(name, src string) (*Program, error) {
	chunk, err := parse.Parse(strings.NewReader(src), name)
	if err != nil {
		return nil, fmt.Errorf("script %s: %w", name, err)
	}
	proto, err := lua.Compile(chunk, name)
	if err != nil {
		return nil, fmt.Errorf("script %s: %w", name, err)
	}
	p := &Program{name: name, proto: proto}

	s := newSandbox()
	defer s.L.Close()
	env, err := s.load(p)
	if err != nil {
		return nil, err
	}
	if err := checkHooks(name, env); err != nil {
		return nil, err
	}
	return p, nil
}

// checkHooks refuses a global named on_* that isn't a hook the world calls,
// or is one but isn't a function: either way it would silently never run.
func checkHooks(name string, env *lua.LTable) error {
	var problems []string
	env.ForEach(func(k, v lua.LValue) {
		key, ok := k.(lua.LString)
		if !ok || !strings.HasPrefix(string(key), "on_") {
			return
		}
		switch {
		case !hooks[string(key)]:
			problems = append(problems, fmt.Sprintf("%s is not a hook", key))
		case v.Type() != lua.LTFunction:
			problems = append(problems, fmt.Sprintf("%s is a %s, not a function", key, v.Type()))
		}
	})
	if len(problems) == 0 {
		return nil
	}
	slices.Sort(problems) // ForEach is map order
	return fmt.Errorf("script %s: %s (hooks are %s)", name,
		strings.Join(problems, "; "), strings.Join(slices.Sorted(maps.Keys(hooks)), ", "))
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go mod tidy && go test ./script/`
Expected: PASS. `go.mod` now lists `github.com/yuin/gopher-lua v1.1.2` without `// indirect`.

- [ ] **Step 6: Commit**

```bash
make check
git add go.mod go.sum script/
git commit -m "script: Compile, a sandboxed Lua that proves a script loads

gopher-lua with only base/string/table/math, nothing that loads code or
reaches the host, and math.random gone so every roll can be loaded. A
top level runs under the 10ms deadline and may define only the hooks the
world calls.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: `script.Runtime`

**Files:**
- Create: `script/runtime.go`, `script/runtime_test.go`

**Interfaces:**
- Consumes (Task 1): `newSandbox`, `(*sandbox).call`, `(*sandbox).load`, `hookCall`, `Roller`, `Program`, `Compile`.
- Consumes: `mobile.Instance` (`Id() uuid.UUID`, `Name() string`, `CurHealth int`, `Definition *mobile.Definition` with `MaxHealth int` and — added in this task — `Script string`).
- Produces:
  - `type Foe struct { Name string; IsPlayer bool }`
  - `type Say func(mob *mobile.Instance, text string)`
  - `const MaxFailures = 3`
  - `func NewRuntime(programs map[string]*Program, roller Roller, say Say) (*Runtime, error)`
  - `func (r *Runtime) FightStart(mob *mobile.Instance, foe Foe)`
  - `func (r *Runtime) FightPulse(mob *mobile.Instance, foe Foe)`
  - `func (r *Runtime) Forget(mob *mobile.Instance)`
  - `mobile.Definition.Script string` — canonical `"zone/name"`, empty for no script.

- [ ] **Step 1: Add `Script` to `mobile.Definition`**

In `mobile/definition.go`, after the `Loot []LootEntry` field:

```go
	// Script is the canonical "zone/name" of the Lua this mob runs, empty for
	// none. A name only: the compiled program lives in loader.Content.Scripts,
	// and mobile knows nothing about Lua.
	Script string
```

- [ ] **Step 2: Write the failing tests**

`script/runtime_test.go`:

```go
package script

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/testdice"
)

type line struct{ mob, text string }

type harness struct {
	t    *testing.T
	rt   *Runtime
	dice *testdice.LoadedDice
	said []line
}

// newHarness compiles each script (keyed "zone/name") into one runtime.
func newHarness(t *testing.T, scripts map[string]string) *harness {
	t.Helper()
	h := &harness{t: t, dice: testdice.New()}
	programs := map[string]*Program{}
	for name, src := range scripts {
		p, err := Compile(name, src)
		require.NoError(t, err)
		programs[name] = p
	}
	rt, err := NewRuntime(programs, h.dice, func(mob *mobile.Instance, text string) {
		h.said = append(h.said, line{mob.Name(), text})
	})
	require.NoError(t, err)
	h.rt = rt
	return h
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
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./script/`
Expected: FAIL to build — `undefined: NewRuntime`.

- [ ] **Step 4: Write the implementation**

`script/runtime.go`:

```go
package script

import (
	"fmt"
	"maps"
	"slices"
	"uuid"

	"github.com/rs/zerolog/log"
	lua "github.com/yuin/gopher-lua"
	"github.com/watchmud/watchmud/mobile"
)

// MaxFailures is how many errors or timeouts a program gets before the
// runtime stops calling it, until restart: a broken script must not flood
// the log every combat round.
const MaxFailures = 3

// Foe is who a scripted mob is fighting, as its script sees them.
type Foe struct {
	Name     string
	IsPlayer bool
}

// Say is how me:say reaches the world: the mob's room hears it.
type Say func(mob *mobile.Instance, text string)

// Runtime is the world's one Lua state, every program loaded into it, and
// each scripted mob's memory. It runs on the world goroutine and holds no
// locks. Every method is a no-op for a mob without a script, so call sites
// never check.
type Runtime struct {
	sb       *sandbox
	roller   Roller
	say      Say
	programs map[string]*loaded
	memory   map[uuid.UUID]*lua.LTable
	// logFailure reports one failed call; a field so a test can see them.
	logFailure func(err error)
}

type loaded struct {
	program  *Program
	env      *lua.LTable
	failures int
}

// NewRuntime loads each program into one Lua state. The programs were proved
// by Compile, so an error here means the content changed underneath us.
func NewRuntime(programs map[string]*Program, roller Roller, say Say) (*Runtime, error) {
	r := &Runtime{
		sb:       newSandbox(),
		roller:   roller,
		say:      say,
		programs: make(map[string]*loaded),
		memory:   make(map[uuid.UUID]*lua.LTable),
	}
	r.logFailure = func(err error) { log.Error().Err(err).Msg("script failed") }
	for _, name := range slices.Sorted(maps.Keys(programs)) {
		env, err := r.sb.load(programs[name])
		if err != nil {
			r.sb.L.Close()
			return nil, err
		}
		r.programs[name] = &loaded{program: programs[name], env: env}
	}
	return r, nil
}

// FightStart is on_fight_start: mob has just gone from not fighting to
// fighting foe.
func (r *Runtime) FightStart(mob *mobile.Instance, foe Foe) {
	r.fire(mob, "on_fight_start", foe)
}

// FightPulse is on_fight_pulse: mob has just swung at foe, and neither of
// them is dead.
func (r *Runtime) FightPulse(mob *mobile.Instance, foe Foe) {
	r.fire(mob, "on_fight_pulse", foe)
}

// Forget drops a mob's memory. The world calls it when the mob leaves the
// world, so memory lives exactly as long as the mob.
func (r *Runtime) Forget(mob *mobile.Instance) {
	delete(r.memory, mob.Id())
}

func (r *Runtime) fire(mob *mobile.Instance, hook string, foe Foe) {
	lp := r.programs[mob.Definition.Script]
	if lp == nil || lp.failures >= MaxFailures {
		return
	}
	fn, ok := lp.env.RawGetString(hook).(*lua.LFunction)
	if !ok {
		return
	}
	call := &hookCall{
		roller: r.roller,
		say:    func(text string) { r.say(mob, text) },
	}
	err := r.sb.call(fn, call, r.me(mob, call), r.foe(foe))
	if err == nil {
		return
	}
	lp.failures++
	r.logFailure(fmt.Errorf("script %s, mob %s, %s (failure %d of %d): %w",
		lp.program.name, mob.Definition.Id, hook, lp.failures, MaxFailures, err))
	if lp.failures == MaxFailures {
		log.Warn().Str("script", lp.program.name).Msg("script disabled until restart")
	}
}

// me is the mob as its script sees it: copies of what it may read, its
// memory, and say -- bound to this one call, so a me kept in memory can't
// speak later.
func (r *Runtime) me(mob *mobile.Instance, call *hookCall) *lua.LTable {
	L := r.sb.L
	me := L.NewTable()
	me.RawSetString("name", lua.LString(mob.Name()))
	me.RawSetString("health", lua.LNumber(mob.CurHealth))
	me.RawSetString("max_health", lua.LNumber(mob.Definition.MaxHealth))
	me.RawSetString("memory", r.memoryOf(mob))
	me.RawSetString("say", L.NewFunction(func(L *lua.LState) int {
		if _, isMe := L.Get(1).(*lua.LTable); !isMe {
			L.RaiseError("say: call it as me:say(text), with a colon")
		}
		text := L.CheckString(2)
		if r.sb.current != call {
			L.RaiseError("say: this me belongs to an earlier call")
		}
		call.say(text)
		return 0
	}))
	return me
}

func (r *Runtime) foe(f Foe) *lua.LTable {
	t := r.sb.L.NewTable()
	t.RawSetString("name", lua.LString(f.Name))
	t.RawSetString("is_player", lua.LBool(f.IsPlayer))
	return t
}

func (r *Runtime) memoryOf(mob *mobile.Instance) *lua.LTable {
	m, ok := r.memory[mob.Id()]
	if !ok {
		m = r.sb.L.NewTable()
		r.memory[mob.Id()] = m
	}
	return m
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./script/ ./mobile/`
Expected: PASS. `TestFailuresAreCountedAndDisable/loop` takes about 30ms (three 10ms deadlines).

- [ ] **Step 6: Commit**

```bash
make check
git add mobile/definition.go script/
git commit -m "script: Runtime, the one Lua state and the hooks it calls

on_fight_start and on_fight_pulse with me (copies, memory, say) and foe.
Every call under the deadline, protected and recovered; three failures and
a program is switched off until restart.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: The loader compiles what mobs name

**Files:**
- Create: `loader/script.go`, `loader/script_test.go`
- Modify: `loader/mobfile.go` (`mobEntry`), `loader/content.go` (`Content`, `NewContent`, `loadMobileDefinitions`)

**Interfaces:**
- Consumes (Task 1): `script.Compile(name, src string) (*script.Program, error)`.
- Consumes (Task 2): `mobile.Definition.Script string`.
- Produces: `loader.Content.Scripts map[string]*script.Program`, keyed on canonical `"zone/name"`; `func (c *Content) mobScript(fsys fs.FS, zoneName string, mob mobEntry) (string, error)`.

- [ ] **Step 1: Write the failing tests**

`loader/script_test.go`:

```go
package loader

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const quietScript = `function on_fight_start(me, foe) me:say("hi") end`

// lootContent (loot_test.go) is wrathrock plus a "caves" zone.
func TestMobScript_ownZone(t *testing.T) {
	c := lootContent(t)
	fsys := fstest.MapFS{"caves/scripts/ghoul.lua": {Data: []byte(quietScript)}}

	ref, err := c.mobScript(fsys, "caves", mobEntry{Id: "ghoul", Script: "ghoul"})

	require.NoError(t, err)
	assert.Equal(t, "caves/ghoul", ref)
	require.Contains(t, c.Scripts, "caves/ghoul")
	assert.Equal(t, "caves/ghoul", c.Scripts["caves/ghoul"].Name())
}

func TestMobScript_otherZone(t *testing.T) {
	c := lootContent(t)
	fsys := fstest.MapFS{"wrathrock/scripts/shout.lua": {Data: []byte(quietScript)}}

	ref, err := c.mobScript(fsys, "caves", mobEntry{Id: "ghoul", Script: "wrathrock/shout"})

	require.NoError(t, err)
	assert.Equal(t, "wrathrock/shout", ref)
}

func TestMobScript_noneIsNone(t *testing.T) {
	c := lootContent(t)
	ref, err := c.mobScript(fstest.MapFS{}, "caves", mobEntry{Id: "bat"})
	require.NoError(t, err)
	assert.Empty(t, ref)
	assert.Empty(t, c.Scripts)
}

// Two mobs naming one script compile it once.
func TestMobScript_sharedCompilesOnce(t *testing.T) {
	c := lootContent(t)
	fsys := fstest.MapFS{"caves/scripts/ghoul.lua": {Data: []byte(quietScript)}}
	_, err := c.mobScript(fsys, "caves", mobEntry{Id: "a", Script: "ghoul"})
	require.NoError(t, err)
	first := c.Scripts["caves/ghoul"]

	_, err = c.mobScript(fsys, "caves", mobEntry{Id: "b", Script: "ghoul"})
	require.NoError(t, err)
	assert.Same(t, first, c.Scripts["caves/ghoul"])
}

func TestMobScript_errors(t *testing.T) {
	fsys := fstest.MapFS{
		"caves/scripts/broken.lua": {Data: []byte(`function on_fight_start(`)},
		"caves/scripts/typo.lua":   {Data: []byte(`function on_fightstart() end`)},
	}
	for _, tc := range []struct{ script, want string }{
		{"missing", "caves/scripts/missing.lua"},
		{"nowhere/ghoul", "zone not found"},
		{"caves/", "names no script"},
		{"broken", "caves/broken"},
		{"typo", "on_fightstart"},
	} {
		t.Run(tc.script, func(t *testing.T) {
			_, err := lootContent(t).mobScript(fsys, "caves", mobEntry{Id: "ghoul", Script: tc.script})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "caves/ghoul", "names the mob")
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./loader/ -run TestMobScript`
Expected: FAIL to build — `c.mobScript undefined`, `unknown field Script in struct literal of type mobEntry`.

- [ ] **Step 3: Write the implementation**

In `loader/mobfile.go`, add to `mobEntry` after `Loot`:

```go
	// Script names the Lua this mob runs: a bare name is this zone's
	// scripts/<name>.lua, "zone/name" any other zone's. See script.go.
	Script string `json:"script"`
```

In `loader/content.go`, add to `Content`:

```go
	// Scripts is every script some mob names, compiled, by "zone/name".
	Scripts map[string]*script.Program
```

and in `NewContent` initialise it alongside `Zones`:

```go
		Scripts:  make(map[string]*script.Program),
```

(import `"github.com/watchmud/watchmud/script"`). In `loadMobileDefinitions`, after the `loot` block:

```go
			scriptRef, err := c.mobScript(fsys, zonename, mob)
			if err != nil {
				return err
			}
```

and after `defn.Loot = loot`:

```go
			defn.Script = scriptRef
```

`loader/script.go`:

```go
package loader

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/watchmud/watchmud/script"
)

// mobScript resolves a mob's "script" -- bare for its own zone, "zone/name"
// for any other, the same rule loot uses -- and compiles it the first time
// anything names it. It returns the canonical "zone/name", empty for no
// script. Anything wrong fails startup: a script that can't load would
// otherwise be a mob that silently never speaks. Only named scripts are
// compiled; a .lua nothing names is ignored.
func (c *Content) mobScript(fsys fs.FS, zoneName string, mob mobEntry) (string, error) {
	if mob.Script == "" {
		return "", nil
	}
	zoneId, name := zoneName, mob.Script
	if z, n, found := strings.Cut(mob.Script, "/"); found {
		zoneId, name = z, n
	}
	if name == "" {
		return "", fmt.Errorf("mob %s/%s: script %q names no script", zoneName, mob.Id, mob.Script)
	}
	if _, ok := c.Zones[zoneId]; !ok {
		return "", fmt.Errorf("mob %s/%s: script %q: zone not found", zoneName, mob.Id, mob.Script)
	}
	ref := zoneId + "/" + name
	if _, done := c.Scripts[ref]; done {
		return ref, nil
	}
	file := path.Join(zoneId, "scripts", name+".lua")
	src, err := fs.ReadFile(fsys, file)
	if err != nil {
		return "", fmt.Errorf("mob %s/%s: script %q: %w", zoneName, mob.Id, mob.Script, err)
	}
	p, err := script.Compile(ref, string(src))
	if err != nil {
		return "", fmt.Errorf("mob %s/%s: %w", zoneName, mob.Id, err)
	}
	c.Scripts[ref] = p
	return ref, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./loader/`
Expected: PASS, including the existing content tests (no content names a script yet).

- [ ] **Step 5: Commit**

```bash
make check
git add loader/
git commit -m "loader: compile the script a mob names, or fail startup

\"script\" in mobs.json resolves like loot (bare is the mob's zone,
zone/name any other) to world/<zone>/scripts/<name>.lua, compiled once
into Content.Scripts.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: The world fires the hooks

**Files:**
- Create: `world/scripts.go`, `world/scripts_test.go`, `testcontent/world/wrathrock/scripts/heckler.lua`, `telnet/render_script_test.go`
- Modify: `world/world.go` (field, `New`, `RemoveMobile`), `world/h_kill.go`, `world/mobile_activity.go` (`doMobAggro`), `world/violence.go` (`DoViolence`), `testcontent/world/wrathrock/mobs.json`, `testcontent/world/wrathrock/instructions.json`, `telnet/render_test.go:304`

**Interfaces:**
- Consumes (Task 2): `script.NewRuntime`, `(*Runtime).FightStart`, `FightPulse`, `Forget`, `script.Foe`.
- Consumes (Task 3): `loader.Content.Scripts`.
- Produces: `func (w *World) startFight(attacker, defender combat.Combatant) error`, `func foeOf(c combat.Combatant) script.Foe`, `func (w *World) mobSays(mob *mobile.Instance, text string)`.

- [ ] **Step 1: Add the scripted test mob**

`testcontent/world/wrathrock/scripts/heckler.lua`:

```lua
-- The test world's one scripted mob. Its opener proves memory: the first
-- fight gets "Come on then", every later one with the same heckler "You again".
local taunts = { "Is that all?", "My grandmother hits harder." }

function on_fight_start(me, foe)
  if me.memory.met then
    me:say("You again, " .. foe.name .. "?")
  else
    me.memory.met = true
    me:say("Come on then, " .. foe.name .. "!")
  end
end

function on_fight_pulse(me, foe)
  if chance(50) then
    me:say(pick(taunts))
  end
end
```

Append to the array in `testcontent/world/wrathrock/mobs.json`:

```json
  {
    "id": "heckler",
    "name": "Heckler",
    "short_description": "A heckler.",
    "description_in_room": "A heckler leans on the counter, smirking.",
    "aliases": [ "heckler" ],
    "max_health": 25,
    "wandering_definition": { "can_wander": false },
    "ac": 10,
    "aggressive": false,
    "script": "heckler"
  }
```

Append to `testcontent/world/wrathrock/instructions.json` (it lives in the general store, a room no other test visits, so no existing `look` changes):

```json
  {
    "type": "CreateMobile",
    "mobile_id": "heckler",
    "room_id": "general_store",
    "instance_max": 1
  }
```

Run: `make check`
Expected: PASS — the heckler loads and compiles, and nothing calls its hooks yet.

- [ ] **Step 2: Write the failing world tests**

`world/scripts_test.go`:

```go
package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
	"github.com/watchmud/watchmud/testdice"
)

// The heckler (testcontent/world/wrathrock/scripts/heckler.lua) in the
// general store, with the player and a second pair of ears beside him.
type scriptsSuite struct {
	worldTestSuite
	store   *spaces.Room
	heckler *mobile.Instance
	ears    *player.Recorder
	dice    *testdice.LoadedDice
}

func TestScriptsSuite(t *testing.T) {
	suite.Run(t, new(scriptsSuite))
}

func (s *scriptsSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.store = s.w.Zone("wrathrock").Rooms["general_store"]
	heckler, found := s.store.FindMobile("heckler")
	s.Require().True(found)
	s.heckler = heckler

	s.w.movePlayer(s.p, rules.DirectionNone, s.store)
	s.ears = &player.Recorder{}
	s.w.PlacePlayer(player.NewTestPlayer(uuid.New(), "listener", s.ears), s.store)

	s.dice = testdice.New()
	s.w.roller = s.dice
	s.r.Clear()
}

// said is every line the heckler's room heard, in order.
func (s *scriptsSuite) said() []string {
	var lines []string
	for _, m := range s.ears.Sent {
		if e, ok := m.(event.Said); ok {
			lines = append(lines, e.Speaker+": "+e.Value)
		}
	}
	return lines
}

func (s *scriptsSuite) kill() {
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Kill{Target: "heckler"})))
}

// kill: "Ok." to the killer first, then the opener, which the room hears.
func (s *scriptsSuite) TestKillFiresTheOpener() {
	s.kill()

	s.Assert().IsType(event.Attacking{}, s.r.Sent[0])
	s.Assert().Equal(event.Said{Speaker: "Heckler", Value: "Come on then, testdood!"}, sent[event.Said](s.T(), s.r, 1))
	s.Assert().Equal([]string{"Heckler: Come on then, testdood!"}, s.said())
}

// Aggro starts a fight too, and gets the same opener.
func (s *scriptsSuite) TestAggroFiresTheOpener() {
	s.heckler.Definition.SetFlag(mobile.Aggressive)

	s.w.DoMobileActivity()

	s.Require().True(s.w.fightLedger.IsFighting(s.heckler))
	s.Assert().Equal([]string{"Heckler: Come on then, testdood!"}, s.said())
}

// Memory is the heckler's own, and lasts as long as he does: a second fight
// with him is "You again", a fresh heckler after a reset starts over.
func (s *scriptsSuite) TestMemoryLastsAsLongAsTheMob() {
	s.kill()
	s.w.fightLedger.EndAllFightsWith(s.p.Id())
	s.kill()

	s.w.combatantDied(s.heckler, s.store)
	s.Require().Empty(s.w.Zone("wrathrock").Reset(s.w.occupancy))
	fresh, found := s.store.FindMobile("heckler")
	s.Require().True(found)
	s.Require().NotSame(s.heckler, fresh)
	s.w.fightLedger.EndAllFightsWith(s.p.Id())
	s.kill()

	s.Assert().Equal([]string{
		"Heckler: Come on then, testdood!",
		"Heckler: You again, testdood?",
		"Heckler: Come on then, testdood!",
	}, s.said())
}

// Joining a fight the heckler is already in, and his retargeting when his
// first opponent leaves, are not starts: one opener, not three.
func (s *scriptsSuite) TestNoOpenerWhenAlreadyFighting() {
	otherRec := &player.Recorder{}
	other := player.NewTestPlayer(uuid.New(), "other", otherRec)
	s.w.PlacePlayer(other, s.store)

	s.kill()
	s.Require().NoError(s.w.startFight(other, s.heckler))
	s.w.fightLedger.EndAllFightsWith(s.p.Id()) // he turns on other

	s.Require().Same(other, s.w.fightLedger.GetFight(s.heckler).Fightee)
	s.Assert().Equal([]string{"Heckler: Come on then, testdood!"}, s.said())
}

// oneSwing leaves exactly one fight -- the heckler at the player -- so
// loaded dice can be aimed at it.
func (s *scriptsSuite) oneSwing() {
	s.Require().NoError(s.w.startFight(s.heckler, s.p))
	s.w.fightLedger.EndFight(s.p)
	s.ears.Clear()
	s.r.Clear()
}

// A round: the blow, then the taunt.
func (s *scriptsSuite) TestPulseTauntsAfterTheBlow() {
	s.oneSwing()
	s.dice.Load([]int{1, 0, 0}) // d20 1: a miss; chance(50) 0: yes; pick 0: "Is that all?"

	s.w.DoViolence(5)

	s.Assert().IsType(event.Struck{}, s.ears.Sent[0])
	s.Assert().Equal(event.Said{Speaker: "Heckler", Value: "Is that all?"}, s.ears.Sent[1])
}

// A killing blow gets no taunt. Spare dice are loaded, so a pulse fired by
// mistake would say something rather than fail on empty dice.
func (s *scriptsSuite) TestNoTauntOverTheBody() {
	s.oneSwing()
	s.p.TakeMeleeDamage(s.p.CurrentHealth() - 1)
	s.dice.Load([]int{20, 5, 0, 0, 0, 0})

	s.w.DoViolence(5)

	s.Require().True(s.p.CurrentHealth() == 1 && s.w.occupancy.RoomOfPlayer(s.p) == s.w.DeathRoom, "the player died and revived")
	s.Assert().Empty(s.said())
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./world -run TestScriptsSuite`
Expected: FAIL to build — `s.w.startFight undefined`.

- [ ] **Step 4: Write the implementation**

`world/scripts.go`:

```go
package world

import (
	"github.com/watchmud/watchmud/combat"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/script"
)

// startFight begins a fight between attacker and defender, then gives each
// mob in it that wasn't already fighting its on_fight_start. Everything that
// starts a fight -- kill and aggro -- comes through here, so no opener is
// missed. Joining a fight a mob is already in isn't a start for that mob, and
// neither is the ledger retargeting it after a death.
func (w *World) startFight(attacker, defender combat.Combatant) error {
	attackerWas, defenderWas := w.fightLedger.InFight(attacker), w.fightLedger.InFight(defender)
	if err := w.fightLedger.Fight(attacker, defender); err != nil {
		return err
	}
	if mob, ok := attacker.(*mobile.Instance); ok && !attackerWas {
		w.scripts.FightStart(mob, foeOf(defender))
	}
	if mob, ok := defender.(*mobile.Instance); ok && !defenderWas {
		w.scripts.FightStart(mob, foeOf(attacker))
	}
	return nil
}

// foeOf is a combatant as a script sees them: a name, and whether they're a
// player.
func foeOf(c combat.Combatant) script.Foe {
	_, isPlayer := c.(*player.Player)
	return script.Foe{Name: c.Name(), IsPlayer: isPlayer}
}

// mobSays is a script's me:say. The mob's room hears it the way it hears a
// player's say.
func (w *World) mobSays(mob *mobile.Instance, text string) {
	w.mobileRoom(mob).Send(event.Said{Speaker: mob.Name(), Value: text})
}
```

In `world/world.go`, add the field to `World` after `fightLedger`:

```go
	// scripts runs the Lua mobs name; see world/scripts.go.
	scripts *script.Runtime
```

In `New`, after the `w = &World{...}` literal and before `w.reservedNames = ...`:

```go
	if w.scripts, err = script.NewRuntime(c.Scripts, roller, w.mobSays); err != nil {
		return nil, fmt.Errorf("building world: %w", err)
	}
```

and change `RemoveMobile` to:

```go
func (w *World) RemoveMobile(mob *mobile.Instance) {
	w.occupancy.RemoveMobile(mob)
	w.scripts.Forget(mob)
}
```

(import `"github.com/watchmud/watchmud/script"` in world.go).

In `world/h_kill.go`, replace the block from `if err := w.fightLedger.Fight(...` to the final `msg.Player.Send(event.Attacking{...})` with:

```go
	// "Ok." first, so a scripted mob's opener answers it rather than coming
	// before it. The ledger can't refuse here -- IsFighting was checked above
	// -- so "Ok." followed by a failure doesn't happen in practice.
	msg.Player.Send(event.Attacking{Target: mobileInstance.Name()})
	if err := w.startFight(msg.Player, mobileInstance); err != nil {
		log.Error().Str("playerName", msg.Player.Name()).Str("target", mobileInstance.Name()).Err(err).Msg("kill: couldn't start the fight")
		msg.Fail(event.InternalError)
	}
```

In `world/mobile_activity.go` `doMobAggro`, change `w.fightLedger.Fight(mob, players[0])` to `w.startFight(mob, players[0])`.

In `world/violence.go` `DoViolence`, between the `if fightResult.WasHit { ... w.wearFromBlow(...) }` block and `if isDead {`:

```go
			// A scripted mob gets its say after the room has seen the blow,
			// and not over a body: a killing blow gets no taunt.
			if mob, ok := fight.Fighter.(*mobile.Instance); ok && !isDead && !fight.Fighter.Dead() {
				w.scripts.FightPulse(mob, foeOf(fight.Fightee))
			}
```

(import `"github.com/watchmud/watchmud/mobile"` in violence.go if not already).

- [ ] **Step 5: Run the world tests**

Run: `go test ./world/`
Expected: PASS. If `TestNoTauntOverTheBody` fails its first assertion, read `playerRevives` — it is the assertion that the setup killed the player, not the behaviour under test.

- [ ] **Step 6: Write the render test**

`telnet/render_script_test.go`:

```go
package telnet

import (
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/world"
)

// hecklerHere brings testcontent's scripted heckler from the general store
// into the start room.
func hecklerHere(w *world.World, _ *player.Player, _ *player.Player) {
	heckler, found := w.Zone("wrathrock").Rooms["general_store"].FindMobile("heckler")
	if !found {
		panic("testcontent has no heckler")
	}
	w.RemoveMobile(heckler)
	w.PlaceMobile(heckler, w.StartRoom)
}

var scriptCases = []commandCase{
	{
		name:      "a scripted mob's opener",
		setup:     hecklerHere,
		input:     "kill heckler",
		want:      "Ok.\nHeckler says, \"Come on then, testdood!\".\n",
		wantOther: "Heckler says, \"Come on then, testdood!\".\n",
	},
}
```

In `telnet/render_test.go:304` change `slices.Concat(commandCases, lootCases)` to `slices.Concat(commandCases, lootCases, scriptCases)`.

- [ ] **Step 7: Run everything**

Run: `make check`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add world/ telnet/ testcontent/
git commit -m "world: scripted mobs speak when a fight starts and as it goes

kill and aggro both go through startFight, which gives each mob newly in
a fight its on_fight_start; DoViolence gives a scripted fighter its
on_fight_pulse after the blow, never over a body. A mob's memory goes
when it does.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: The Barrow-King speaks, and the docs

**Files:**
- Create: `content/world/barrow/scripts/barrow_king.lua`
- Modify: `content/world/barrow/mobs.json` (the `barrow_king` entry), `CLAUDE.md`, `ROADMAP.md`

**Interfaces:**
- Consumes: everything above; no new Go.

- [ ] **Step 1: Write the King's script**

`content/world/barrow/scripts/barrow_king.lua` (placeholder lines, for the user to rewrite):

```lua
-- The Barrow-King. Placeholder lines: rewrite them in his voice.
local openers = {
  "Who wakes the king beneath the oak?",
  "Kneel, %s. The barrow keeps what it takes.",
}
local taunts = {
  "Hah! You cannot defeat me!",
  "I have outlasted better than you.",
  "Your bones will guard my door.",
}

function on_fight_start(me, foe)
  me:say(string.format(pick(openers), foe.name))
end

function on_fight_pulse(me, foe)
  if chance(15) then
    local line = pick(taunts)
    if line ~= me.memory.last then
      me:say(line)
      me.memory.last = line
    end
  end
end
```

In `content/world/barrow/mobs.json`, add to the `barrow_king` object, after `"aggressive": true,`:

```json
    "script": "barrow_king",
```

- [ ] **Step 2: Prove the real content loads**

The loader's existing tests load the real `content/` directory
(`loader/content_hollowfield_test.go` and `loader/durability_test.go` call
`LoadContent(os.DirFS("../content"))`), so a script that fails to compile fails them.

Run: `go test ./loader/`
Expected: PASS. To see it fail first, temporarily break the King's script (e.g. change
`function on_fight_start` to `function on_fightstart`): `go test ./loader/` must fail
naming `barrow/barrow_king`. Put it back.

- [ ] **Step 3: Update CLAUDE.md**

Add a section after "### Player death" and before "### Definition vs Instance":

```markdown
### Scripts (Lua)

**Go is the engine; a script decides *when*.** A script composes actions the engine
already has -- today, one: `me:say` -- and never does the math. An action a script needs
that the engine lacks is a Go feature first.

A mob names a script in mobs.json, `"script": "barrow_king"` (bare is its zone,
`"zone/name"` any other), and the file is `world/<zone>/scripts/<name>.lua`. The loader
compiles every named script (`script.Compile`) into `loader.Content.Scripts`; a missing
file, a syntax error, a top level that errors or runs too long, or an unknown `on_` hook
fails startup. `world.New` loads them into one `script.Runtime`: one gopher-lua state, on
the world goroutine, no locks.

Hooks, both optional: `on_fight_start(me, foe)` (fired by `World.startFight` -- which
`kill` and aggro both use -- for each mob that wasn't already fighting) and
`on_fight_pulse(me, foe)` (from `DoViolence`, after the blow, never over a body). `me` is
copies (`name`, `health`, `max_health`), `me.memory` (a table per mob instance, dropped
with the mob in `World.RemoveMobile`), and `me:say`, bound to that one call. `foe` is
`name` and `is_player`. `chance(pct)` and `pick(list)` roll through `w.roller`, only
inside a hook; `math.random` is gone so tests can load the dice.

**A bad script can't hurt the server.** Every call runs under `script.CallTimeout`
(10ms), a capped call stack and registry, gopher-lua's protected call and a `recover`.
A failure is logged and the mob carries on; after `script.MaxFailures` (3) the program is
switched off until restart. Only base, string, table and math are open, minus anything
that loads code, reaches another program's globals, or prints.

Adding a hook: a `Runtime` method that calls `fire` with its name, the name in `hooks`
(script/program.go), the Go call site, and a test in `world/scripts_test.go`.
```

- [ ] **Step 4: Update ROADMAP.md**

In the "First week" list, replace `- Mob taunts, the first Lua (below, "No scripting language").` with:

```markdown
- ~~Mob taunts, the first Lua (below, "No scripting language").~~ Done 2026-09-28:
  `script/`, two fight hooks and `me:say`; the Barrow-King has a script.
```

In "Known problems", at the end of the "**No scripting language.**" entry, add:

```markdown
  **Taunts are done** (2026-09-28; spec in `docs/superpowers/specs/`, and CLAUDE.md
  "Scripts (Lua)"). Next is the King's half-health script, which brings `wait()` over
  coroutines and a "health crossed a line" hook.
```

- [ ] **Step 5: Commit**

```bash
make check
git add content/ CLAUDE.md ROADMAP.md
git commit -m "the Barrow-King has something to say

His script is the first real one: an opener and the odd taunt, never
the same line twice running. Placeholder lines. CLAUDE.md gains
\"Scripts (Lua)\"; ROADMAP strikes taunts and names the next case.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
