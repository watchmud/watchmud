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
