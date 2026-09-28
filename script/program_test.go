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
