// Package script runs the Lua a builder attaches to a mob. Go is the engine:
// a script decides *when*, from actions the engine already has, and never how
// the math works. See CLAUDE.md, "Scripts (Lua)".
package script

import (
	"context"
	"fmt"
	"regexp"
	"strings"
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

	// maxBuiltString bounds what string.rep may build. It runs in Go, where
	// the deadline can't interrupt it, so the bound is checked before the
	// work rather than noticed after it.
	maxBuiltString = 4 * 1024
)

// MaxWait and MaxWaitPerCall bound wait(): one wait is at most 10 seconds,
// a hook runs at most 30 in all. A waiting mob's other hooks don't fire,
// so a hook that waited forever would silence it for good.
const (
	MaxWait        = 10
	MaxWaitPerCall = 30
)

// patternFunctions are removed from the string library: gopher-lua's pattern
// matcher runs in Go, where the deadline can't reach, and a pathological
// pattern backtracks for minutes. A taunt needs format, sub, upper and "..";
// a script that needs matching is a Go feature first.
var patternFunctions = []string{"find", "match", "gmatch", "gfind", "gsub", "dump"}

// bigFormatField is a format directive with a width or precision of three
// digits or more. Go's fmt pads each one out in full, in Go, so "%999999d"
// is a megabyte the deadline never sees.
var bigFormatField = regexp.MustCompile(`%[-+ #0]*(\d{3,}|\d*\.\d{3,})`)

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
	// says counts this call's says against MaxSaysPerCall.
	says int
	// summoned counts this call's summons against MaxSummonsPerCall.
	summoned int
	// swept counts this call's sweeps against MaxSweepsPerCall.
	swept int
	// fled is whether this call has tried me:flee() already.
	fled bool
	// took is whether this call has used its one me:take().
	took bool
	// waited is this run's seconds of waiting so far, against MaxWaitPerCall
	waited int
	// wait is the one it's in now, for Tick to count down
	wait int
}

// sandbox is one Lua state with only the safe libraries open, plus chance and
// pick.
type sandbox struct {
	L       *lua.LState
	current *hookCall
	// protected counts the pcalls and xpcalls in progress; wait refuses
	// inside one.
	protected int
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

	// The string table is also strings' metatable, so these cover method
	// calls too: ("x"):find is gone and ("x"):rep is capped.
	strLib := g.RawGetString("string").(*lua.LTable)
	for _, name := range patternFunctions {
		strLib.RawSetString(name, lua.LNil)
	}
	strLib.RawSetString("rep", s.L.NewFunction(cappedRep))
	format := strLib.RawGetString("format").(*lua.LFunction).GFunction
	strLib.RawSetString("format", s.L.NewFunction(func(L *lua.LState) int {
		if bigFormatField.MatchString(L.CheckString(1)) {
			L.RaiseError("format: widths and precisions stop at 99")
		}
		return format(L)
	}))
	// getmetatable("") would otherwise hand a script the one string table
	// every program's method calls go through.
	strLib.RawSetString("__metatable", lua.LString("locked"))

	// wait can't pause through pcall: gopher-lua's pcall takes a yield for a
	// return, and the hook would carry on at once. Count the protected calls
	// in progress; wait refuses inside one.
	for _, name := range []string{"pcall", "xpcall"} {
		protect := g.RawGetString(name).(*lua.LFunction).GFunction
		g.RawSetString(name, s.L.NewFunction(func(L *lua.LState) int {
			s.protected++
			defer func() { s.protected-- }()
			return protect(L)
		}))
	}

	g.RawSetString("chance", s.L.NewFunction(s.chance))
	g.RawSetString("pick", s.L.NewFunction(s.pick))
	g.RawSetString("wait", s.L.NewFunction(s.wait))
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

// wait(seconds) suspends the hook in progress: it yields out of the hook's
// coroutine, and Runtime.Tick resumes it that many pulses -- seconds -- later.
func (s *sandbox) wait(L *lua.LState) int {
	n := L.CheckNumber(1)
	h := s.running(L, "wait")
	if s.protected > 0 {
		L.RaiseError("wait: can't pause inside pcall or xpcall")
	}
	secs := int(n)
	if lua.LNumber(secs) != n || secs < 1 || secs > MaxWait {
		L.RaiseError("wait: %v is not a whole number of seconds from 1 to %d", n, MaxWait)
	}
	if h.waited+secs > MaxWaitPerCall {
		L.RaiseError("wait: more than %d seconds of waiting in one call", MaxWaitPerCall)
	}
	h.waited += secs
	h.wait = secs
	return L.Yield()
}

// cappedRep is string.rep, refusing a result longer than maxBuiltString.
func cappedRep(L *lua.LState) int {
	str := L.CheckString(1)
	n := L.CheckInt(2)
	if n <= 0 || str == "" {
		L.Push(lua.LString(""))
		return 1
	}
	if n > maxBuiltString/len(str) {
		L.RaiseError("rep: the result would be longer than %d bytes", maxBuiltString)
	}
	L.Push(lua.LString(strings.Repeat(str, n)))
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

// resume runs th -- starting fn with args the first time, carrying on where
// it yielded after that -- under a fresh CallTimeout, with h as the hook in
// progress. done is false when the hook yielded (wait) rather than
// finished. An error, a timeout, a stack overflow or a panic come back as an
// error; none of them escape.
func (s *sandbox) resume(th *lua.LState, fn *lua.LFunction, h *hookCall, args ...lua.LValue) (done bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), CallTimeout)
	defer cancel()
	th.SetContext(ctx)
	s.current = h
	defer func() {
		s.current = nil
		th.RemoveContext()
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	state, err, _ := s.L.Resume(th, fn, args...)
	switch state {
	case lua.ResumeError:
		return false, err
	case lua.ResumeYield:
		return false, nil
	}
	return true, nil
}

// load runs p's top level in a fresh environment table and returns that
// table: the program's own globals, which is where its hooks end up. The
// table starts as a copy of the globals, with its own copies of the library
// tables and _G pointing at itself, so whatever a program does to string,
// pick or _G -- at load or in a hook -- it does only to itself.
func (s *sandbox) load(p *Program) (*lua.LTable, error) {
	env := s.L.NewTable()
	s.L.G.Global.ForEach(func(k, v lua.LValue) {
		if lib, ok := v.(*lua.LTable); ok {
			v = copyTable(s.L, lib)
		}
		env.RawSet(k, v)
	})
	env.RawSetString("_G", env)

	fn := s.L.NewFunctionFromProto(p.proto)
	fn.Env = env // functions the script defines inherit it
	if err := s.call(fn, nil); err != nil {
		return nil, fmt.Errorf("script %s: %w", p.name, err)
	}
	return env, nil
}

// copyTable is a shallow copy: a library's functions are shared, the table
// holding them is not.
func copyTable(L *lua.LState, t *lua.LTable) *lua.LTable {
	c := L.NewTable()
	t.ForEach(func(k, v lua.LValue) { c.RawSet(k, v) })
	return c
}
