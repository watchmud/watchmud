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
	"on_hear":        true,
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
