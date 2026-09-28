package script

import (
	"fmt"
	"maps"
	"slices"
	"uuid"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/mobile"
	lua "github.com/yuin/gopher-lua"
)

// MaxFailures is how many errors or timeouts a program gets before the
// runtime stops calling it, until restart: a broken script must not flood
// the log every combat round.
const MaxFailures = 3

// MaxSaysPerCall and MaxSayLength bound what one hook call can put in front
// of a room. Each say is a Send to every player there, and a connection whose
// queue fills is hung up on: a script saying in a loop would otherwise
// disconnect the room before the deadline stopped it.
const (
	MaxSaysPerCall = 2
	MaxSayLength   = 300
)

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
		if len(text) > MaxSayLength {
			L.RaiseError("say: %d bytes is longer than %d", len(text), MaxSayLength)
		}
		if call.says == MaxSaysPerCall {
			L.RaiseError("say: more than %d says in one call", MaxSaysPerCall)
		}
		call.says++
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
