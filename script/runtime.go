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

// MaxSummonsPerCall and MaxLiveSummons bound what a script can bring into the
// world: per hook call, across every me:summon in it, and alive at once per
// summoner. Over either, fewer arrive -- a limit, not an error -- so a boss
// that summons every round simply stops getting more.
const (
	MaxSummonsPerCall = 4
	MaxLiveSummons    = 4
)

// Foe is who a scripted mob is fighting, as its script sees them.
type Foe struct {
	Name     string
	IsPlayer bool
}

// Actions is the engine as a script reaches it - what me's methods call.
// The world fills it in; a test fills it with recorders.
type Actions struct {
	// Say is me:say: the mob's room hears it
	Say func(mob *mobile.Instance, text string)
	// Summon is me:summon, already checked against the mob's Summons and
	// cut to the caps: put count of def in the world and answer how many came.
	Summon func(summoner *mobile.Instance, def *mobile.Definition, count int) int
	// Summons is me.summons: how many of the mob's summons are alive.
	Summons func(summoner *mobile.Instance) int
}

// Runtime is the world's one Lua state, every program loaded into it, and
// each scripted mob's memory. It runs on the world goroutine and holds no
// locks. Every method is a no-op for a mob without a script, so call sites
// never check.
type Runtime struct {
	sb       *sandbox
	roller   Roller
	actions  Actions
	programs map[string]*loaded
	memory   map[uuid.UUID]*lua.LTable
	waiting  []*hookRun // in the order they began waiting
	// logFailure reports one failed call; a field so a test can see them.
	logFailure func(err error)
}

type loaded struct {
	program  *Program
	env      *lua.LTable
	failures int
}

// hookRun is one hook running on one mob, from its start to its end, across
// any waits: its own coroutine, the call its caps and its me belonging to.
type hookRun struct {
	mob    *mobile.Instance
	lp     *loaded
	hook   string
	fn     *lua.LFunction
	th     *lua.LState
	call   *hookCall
	me     *lua.LTable
	pulses int // left to wait
}

// NewRuntime loads each program into one Lua state. The programs were proved
// by Compile, so an error here means the content changed underneath us.
func NewRuntime(programs map[string]*Program, roller Roller, actions Actions) (*Runtime, error) {
	r := &Runtime{
		sb:       newSandbox(),
		roller:   roller,
		actions:  actions,
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
		say:    func(text string) { r.actions.Say(mob, text) },
	}
	th, _ := r.sb.L.NewThread()
	run := &hookRun{mob: mob, lp: lp, hook: hook, fn: fn, th: th, call: call,
		me: r.me(mob, call)}
	r.run(run, run.me, r.foe(foe))
}

// run starts or carries on a hook run -- args start it -- and deals with
// how it ended.
func (r *Runtime) run(run *hookRun, args ...lua.LValue) {
	done, err := r.sb.resume(run.th, run.fn, run.call, args...)
	switch {
	case err != nil:
		r.fail(run, err)
	case !done:
		run.pulses = run.call.wait
		r.waiting = append(r.waiting, run)
	}
}

// Tick is one pulse for every hook run waiting on wait(), in the order they
// began waiting. One that comes due carries on, with me's health and summons
// read fresh. inFight is the world's answer to "is this mob still in a fight?" (See Task 3)
func (r *Runtime) Tick(inFight func(*mobile.Instance) bool) {
	var due, still []*hookRun
	for _, run := range r.waiting {
		run.pulses--
		if run.pulses > 0 {
			still = append(still, run)
		} else {
			due = append(due, run)
		}
	}
	r.waiting = still
	for _, run := range due {
		r.refresh(run.mob, run.me)
		r.run(run)
	}
}

// refresh re-reads what me compies that can change while a hook waits.
func (r *Runtime) refresh(mob *mobile.Instance, me *lua.LTable) {
	me.RawSetString("health", lua.LNumber(mob.CurHealth))
	me.RawSetString("summons", lua.LNumber(r.actions.Summons(mob)))
}

// fail counts a failed run against its program, switching the program off at MaxFailures.
func (r *Runtime) fail(run *hookRun, err error) {
	lp := run.lp
	lp.failures++
	r.logFailure(fmt.Errorf("script %s, mob %s, %s (failure %d of %d): %w",
		lp.program.name,
		run.mob.Definition.Id,
		run.hook,
		lp.failures,
		MaxFailures,
		err))
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
	me.RawSetString("summons", lua.LNumber(r.actions.Summons(mob)))
	me.RawSetString("summon", L.NewFunction(func(L *lua.LState) int {
		if _, isMe := L.Get(1).(*lua.LTable); !isMe {
			L.RaiseError("summon: call it as me:summon(id, count), with a colon")
		}
		id := L.CheckString(2)
		count := L.CheckInt(3)
		if r.sb.current != call {
			L.RaiseError("summon: this me belongs to an earlier call")
		}
		def := summonable(mob, id)
		if def == nil {
			L.RaiseError("summon: %s may not summon %q -- see \"summons\" in mobs.json", mob.Definition.Id, id)
		}
		if count < 1 {
			L.RaiseError("summon: a count of %d", count)
		}
		count = min(count, MaxSummonsPerCall-call.summoned, MaxLiveSummons-r.actions.Summons(mob))
		got := 0
		if count > 0 {
			got = r.actions.Summon(mob, def, count)
			call.summoned += got
		}
		L.Push(lua.LNumber(got))
		return 1
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

// summonable is the definition id names among those mob may summon: its bare
// id, or "zone/id", either way mobs.json may have named it.
func summonable(mob *mobile.Instance, id string) *mobile.Definition {
	for _, d := range mob.Definition.Summons {
		if id == d.Id || id == d.ZoneId+"/"+d.Id {
			return d
		}
	}
	return nil
}
