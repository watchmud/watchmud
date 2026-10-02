# Abilities from gear: `cast heal` -- Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **This one is hand-implemented**: the user writes the code, Claude walks through each
> task and reviews it before the commit.

**Goal:** Equip a censer, `cast heal bob`, and Bob gets health back -- mid-fight
included -- paid for in mana and a cooldown, with the ability defined in a rules file and
its effect in Go.

**Architecture:** `rules.Ability` (from `content/rules/abilities.json`) holds the numbers;
`object.Definition.Abilities` says which items grant which; `Equipment.Abilities()`
derives what a player can cast from the slots on every read. `world/h_cast.go` runs the
checks and spends; `world/abilities.go` maps each ability id to a Go effect, and
`world.New` refuses an ability with none. Mana sits beside health on the player and the
record; cooldowns are in memory only.

**Tech Stack:** Go, testify (`assert`/`require`/`suite`), the existing telnet renderer.

**Spec:** `docs/superpowers/specs/2026-10-01-abilities-heal-design.md`

## Global Constraints

- `make check` (fmt-check, vet, test) green before every commit; one commit per task, on master.
- Gear is the truth: no "known abilities" field on `player.Player` or `player.Record`.
- Nothing mechanical branches on a role.
- Max mana is flat **100**; mana regen is **5% of max, at least 1**, per regen pulse, **in a fight or out**.
- Heal: **20 mana, 10s cooldown, `base 10, per_power 2`**, target `friend`.
- A cast that passes the checks **always spends** mana and cooldown, even on an unhurt target.
- Target kinds: `self`, `friend`, `foe`, `none`. Heal is `friend`: empty target is yourself; otherwise a player **in your room**.
- `CurMana` on the record is a `*int`: absent reads as full.
- Cooldowns are never saved.
- Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Three refinements of the spec, decided while planning

1. **`Equipment.Abilities()` returns a map, not an ordered list.** Equipment would need
   the catalog's ability order to sort, and the player's catalog in tests is
   `rules.NewTestCatalog()`, which has no abilities. So equipment answers
   `map[abilityId]Grant`, and the one place order matters -- the `abilities` listing --
   walks `Catalog.AbilityList()` and looks each up. Same behaviour the spec describes.
2. **The prompt shows mana only when `MaxMana > 0`.** Every real player has 100, so in
   play it is always there; but the ~40 test fixtures that build `event.Prompt{CurrentHealth,
   MaxHealth}` keep rendering `<100/100hp> ` unchanged, and the bots' patterns take the
   mana part as optional. Much less churn, same result on the wire.
3. **`NOT_GRANTED` reads "Nothing you're wearing lets you cast that."** -- `event.Failed`
   carries a verb and a code, not the ability's name.

## Review Focus

Inputs the spec implies but doesn't spell out, most likely to bite first. Each has a test
in the task named.

1. **Case.** `cast HEAL Otherdood` heals Otherdood: the ability id is lowercased in the
   handler, names go through `NameKey`. (Task 5)
2. **Healing yourself by name.** `cast heal testdood` is the same as `cast heal`. (Task 5)
3. **A player in another room.** `cast heal bob` with Bob elsewhere is `TARGET_NOT_FOUND`
   -- no remote heals -- and spends nothing. (Task 5)
4. **One ability, two items, one broken.** The broken one grants nothing, even if it has
   the higher power. (Task 2)
5. **Out of mana.** A player at 0 mana still sees `0/100m` in the prompt -- the rule is
   `MaxMana > 0`, not `CurrentMana > 0`. (Task 4)

---

## File map

| File | Task | What |
|---|---|---|
| `rules/ability.go` (new) | 1 | `Ability`, `AbilityTarget`, `AbilityAmount`, `MaxMana`, JSON |
| `rules/ability_test.go` (new) | 1 | parsing and refusals |
| `rules/catalog.go` | 1 | `Abilities` map, `abilityOrder`, `SetAbilities`, `AbilityList` |
| `loader/catalog.go` | 1 | read optional `abilities.json` |
| `loader/abilities_test.go` (new) | 1, 2 | real content has heal; unknown grant fails |
| `content/rules/abilities.json`, `testcontent/rules/abilities.json` (new) | 1 | heal |
| `object/definition.go` | 2 | `Abilities []string` |
| `loader/objectfile.go`, `loader/content.go` | 2 | `"abilities"` key, checked against catalog |
| `object/equipment.go`, `object/abilities_test.go` (new) | 2 | `Grant`, `Equipment.Abilities()` |
| `player/player.go`, `player/record.go`, `player/mana_test.go` (new) | 3 | mana, cooldowns, record |
| `mongostore/document.go`, `mongostore/document_test.go` | 3 | `cur_mana` |
| `rules/regen.go`, `world/regen.go`, `world/regen_test.go` | 3 | mana regen |
| `event/events.go`, `server/gameserver.go`, `server/gameserver_test.go` | 4 | prompt mana |
| `telnet/render.go`, `telnet/ansi_test.go` | 4 | `<..hp 80/100m> ` |
| `bot/client.go`, `bot/smoke.go`, `bot/client_test.go` | 4 | optional mana in patterns |
| `command/commands.go`, `event/result.go`, `event/events.go` | 5, 6 | `Cast`, `Abilities`, codes, `Healed`, `Abilities` event |
| `world/world.go` | 5 | `now` clock, effects check in `New` |
| `world/abilities.go` (new), `world/h_cast.go` (new), `world/h_cast_test.go` (new) | 5 | the cast |
| `world/handlers.go` | 5, 6 | two `case`s |
| `telnet/parse.go`, `telnet/render.go`, `telnet/resultcode.go`, `telnet/help.go`, `telnet/render_test.go` | 5, 6 | the player-facing half |
| `testcontent/world/wrathrock/objects.json` | 5 | the test censer grants heal |
| `world/h_abilities.go` (new), `world/h_abilities_test.go` (new) | 6 | listing |
| `content/world/{hollowfield,sample,barrow}/objects.json` | 7 | the grants |
| `CLAUDE.md`, `LEVELS.md`, `content/rules/README.md`, `site/index.html` | 8 | docs |

---

### Task 1: `rules.Ability` and abilities.json

**Files:**
- Create: `rules/ability.go`, `rules/ability_test.go`, `loader/abilities_test.go`,
  `content/rules/abilities.json`, `testcontent/rules/abilities.json`
- Modify: `rules/catalog.go`, `loader/catalog.go`

**Interfaces:**
- Produces: `rules.Ability{Id, Name string; Mana int; Cooldown time.Duration; Target AbilityTarget; Amount AbilityAmount}`,
  `rules.AbilityAmount{Base, PerPower int}` with `For(power int) int`,
  `rules.AbilityTarget` constants `TargetSelf`, `TargetFriend`, `TargetFoe`, `TargetNone`,
  `rules.MaxMana = 100`,
  `(*Catalog).SetAbilities([]*Ability) error`, `(*Catalog).AbilityList() []*Ability`,
  `Catalog.Abilities map[string]*Ability`.

- [ ] **Step 1: Write the failing tests** -- `rules/ability_test.go`

```go
package rules

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAbility_unmarshal(t *testing.T) {
	var a Ability
	require.NoError(t, json.Unmarshal([]byte(`{
		"id": "heal", "name": "heal", "mana": 20, "cooldown": "10s",
		"target": "friend", "amount": {"base": 10, "per_power": 2}
	}`), &a))
	assert.Equal(t, Ability{
		Id: "heal", Name: "heal", Mana: 20, Cooldown: 10 * time.Second,
		Target: TargetFriend, Amount: AbilityAmount{Base: 10, PerPower: 2},
	}, a)
}

func TestAbility_unmarshalBadCooldown(t *testing.T) {
	var a Ability
	assert.Error(t, json.Unmarshal([]byte(`{"id": "heal", "cooldown": "soon"}`), &a))
}

func TestAbility_noCooldownIsZero(t *testing.T) {
	var a Ability
	require.NoError(t, json.Unmarshal([]byte(`{"id": "x", "name": "x", "target": "self"}`), &a))
	assert.Zero(t, a.Cooldown)
}

func TestAbilityAmount_for(t *testing.T) {
	amt := AbilityAmount{Base: 10, PerPower: 2}
	assert.Equal(t, 10, amt.For(0))
	assert.Equal(t, 12, amt.For(1))
	assert.Equal(t, 26, amt.For(8))
}

func testHeal() *Ability {
	return &Ability{Id: "heal", Name: "heal", Mana: 20, Cooldown: 10 * time.Second,
		Target: TargetFriend, Amount: AbilityAmount{Base: 10, PerPower: 2}}
}

func TestCatalog_setAbilities(t *testing.T) {
	c, err := NewTestCatalog()
	require.NoError(t, err)
	zap := &Ability{Id: "zap", Name: "zap", Target: TargetFoe}
	require.NoError(t, c.SetAbilities([]*Ability{testHeal(), zap}))

	assert.Equal(t, "heal", c.Abilities["heal"].Id)
	assert.Equal(t, []*Ability{c.Abilities["heal"], zap}, c.AbilityList(), "declaration order")
}

func TestCatalog_setAbilitiesRefuses(t *testing.T) {
	for name, a := range map[string]*Ability{
		"no id":            {Name: "heal", Target: TargetSelf},
		"no name":          {Id: "heal", Target: TargetSelf},
		"negative mana":    {Id: "heal", Name: "heal", Target: TargetSelf, Mana: -1},
		"negative cooldown": {Id: "heal", Name: "heal", Target: TargetSelf, Cooldown: -time.Second},
		"unknown target":   {Id: "heal", Name: "heal", Target: "everyone"},
		"no target":        {Id: "heal", Name: "heal"},
	} {
		t.Run(name, func(t *testing.T) {
			c, err := NewTestCatalog()
			require.NoError(t, err)
			assert.Error(t, c.SetAbilities([]*Ability{a}))
		})
	}
}

func TestCatalog_setAbilitiesRefusesDuplicates(t *testing.T) {
	c, err := NewTestCatalog()
	require.NoError(t, err)
	assert.Error(t, c.SetAbilities([]*Ability{testHeal(), testHeal()}))
}
```

`loader/abilities_test.go`:

```go
package loader

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The real game has heal, and so does the content the world tests load.
func TestLoadCatalog_abilities(t *testing.T) {
	for _, dir := range []string{"../content/rules", "../testcontent/rules"} {
		cat, err := LoadCatalog(os.DirFS(dir))
		require.NoError(t, err, dir)
		heal, found := cat.Abilities["heal"]
		require.True(t, found, dir)
		assert.Positive(t, heal.Mana, dir)
		assert.Positive(t, heal.Cooldown, dir)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./rules ./loader -run 'Abilit'`
Expected: build failure -- `undefined: Ability`, `undefined: TargetFriend`, ...

- [ ] **Step 3: Implement** -- `rules/ability.go`

```go
package rules

import (
	"encoding/json"
	"fmt"
	"time"
)

// MaxMana is everyone's, flat, the way max health is today. Gear decides what
// you can cast, not how much -- see the abilities spec. If max health ever
// comes from gear, this goes with it.
const MaxMana = 100

// AbilityTarget is what an ability is aimed at. The cast handler resolves the
// target once, by kind, so no effect ever parses a target string.
type AbilityTarget string

const (
	TargetSelf   AbilityTarget = "self"   // always the caster
	TargetFriend AbilityTarget = "friend" // the caster, or a player in their room
	TargetFoe    AbilityTarget = "foe"    // a mob in the caster's room
	TargetNone   AbilityTarget = "none"   // nothing: an ability about the world
)

func (t AbilityTarget) valid() bool {
	switch t {
	case TargetSelf, TargetFriend, TargetFoe, TargetNone:
		return true
	}
	return false
}

// AbilityAmount is how much an ability does at a given power: heal's
// parameter. Other abilities add their own fields to Ability rather than
// reusing this one for something it doesn't mean.
type AbilityAmount struct {
	Base     int `json:"base"`
	PerPower int `json:"per_power"`
}

// For is the amount at this power -- the power of the item granting it.
func (a AbilityAmount) For(power int) int {
	return a.Base + a.PerPower*power
}

// Ability is something gear lets you do, from content/rules/abilities.json.
// The numbers live here; what it does is Go (world/abilities.go), and an
// ability without an effect there fails startup.
type Ability struct {
	Id       string
	Name     string
	Mana     int
	Cooldown time.Duration
	Target   AbilityTarget
	Amount   AbilityAmount
}

func (a *Ability) UnmarshalJSON(data []byte) error {
	var raw struct {
		Id       string        `json:"id"`
		Name     string        `json:"name"`
		Mana     int           `json:"mana"`
		Cooldown string        `json:"cooldown"`
		Target   AbilityTarget `json:"target"`
		Amount   AbilityAmount `json:"amount"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*a = Ability{Id: raw.Id, Name: raw.Name, Mana: raw.Mana, Target: raw.Target, Amount: raw.Amount}
	if raw.Cooldown != "" {
		d, err := time.ParseDuration(raw.Cooldown)
		if err != nil {
			return fmt.Errorf("ability %q cooldown: %w", raw.Id, err)
		}
		a.Cooldown = d
	}
	return nil
}

func (a *Ability) check() error {
	switch {
	case a.Id == "":
		return fmt.Errorf("ability %q: missing id", a.Name)
	case a.Name == "":
		return fmt.Errorf("ability %q: missing name", a.Id)
	case a.Mana < 0:
		return fmt.Errorf("ability %q: mana %d", a.Id, a.Mana)
	case a.Cooldown < 0:
		return fmt.Errorf("ability %q: cooldown %s", a.Id, a.Cooldown)
	case !a.Target.valid():
		return fmt.Errorf("ability %q: unknown target %q", a.Id, a.Target)
	}
	return nil
}
```

In `rules/catalog.go`, add to the `Catalog` struct after `Economy`:

```go
	// Abilities are what gear can let a player do, keyed on id. Assigned by
	// the loader through SetAbilities; none is a game where nobody casts.
	Abilities map[string]*Ability
```

and beside `roleOrder` in the unexported block:

```go
	abilityOrder []*Ability
```

and the two methods, after `RoleList`:

```go
// SetAbilities checks and indexes the abilities, keeping their declaration
// order for listings.
func (c *Catalog) SetAbilities(all []*Ability) error {
	byId := make(map[string]*Ability, len(all))
	for _, a := range all {
		if err := a.check(); err != nil {
			return err
		}
		if _, dup := byId[a.Id]; dup {
			return fmt.Errorf("duplicate ability id %q", a.Id)
		}
		byId[a.Id] = a
	}
	c.Abilities = byId
	c.abilityOrder = all
	return nil
}

// AbilityList returns every ability in the order the content declared them.
func (c *Catalog) AbilityList() []*Ability {
	return c.abilityOrder
}
```

In `loader/catalog.go`, before `return c, nil`:

```go
	// Optional: absent is a game where nobody casts anything.
	abilities, err := readOptionalJSONFile[[]*rules.Ability](rulesFS, "abilities.json")
	if err != nil {
		return nil, err
	}
	if err := c.SetAbilities(abilities); err != nil {
		return nil, err
	}
```

`content/rules/abilities.json` **and** `testcontent/rules/abilities.json` (identical):

```json
[
  {
    "id": "heal",
    "name": "heal",
    "mana": 20,
    "cooldown": "10s",
    "target": "friend",
    "amount": { "base": 10, "per_power": 2 }
  }
]
```

- [ ] **Step 4: Run the tests**

Run: `go test ./rules ./loader`
Expected: PASS

- [ ] **Step 5: `make check`, then commit**

```bash
make check
git add rules/ability.go rules/ability_test.go rules/catalog.go loader/catalog.go loader/abilities_test.go content/rules/abilities.json testcontent/rules/abilities.json
git commit -m "abilities: rules.Ability and abilities.json, with heal

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Gear grants abilities

**Files:**
- Modify: `object/definition.go`, `object/equipment.go`, `loader/objectfile.go`, `loader/content.go`, `loader/abilities_test.go`
- Create: `object/abilities_test.go`

**Interfaces:**
- Consumes: `Catalog.Abilities` (Task 1).
- Produces: `object.Definition.Abilities []string`;
  `object.Grant{Instance *Instance; Power int}`;
  `(*Equipment).Abilities() map[string]Grant`;
  `loader.objectAbilities(zone string, obj objectEntry, cat *rules.Catalog) ([]string, error)`.

- [ ] **Step 1: Write the failing tests** -- `object/abilities_test.go`

```go
package object

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/rules"
)

// a censer-ish thing for this slot, granting heal at this power
func granting(t *testing.T, name string, slot rules.EquipmentSlot, power int) *Instance {
	t.Helper()
	d := NewTestDefinition(t, name, slot, rules.ObjectCategoryOther, rules.ArmorTypeNone)
	d.Abilities = []string{"heal"}
	d.MaxDurability = 10
	inst := NewInstance(uuid.New(), d)
	inst.Power = power
	return inst
}

func newTestEquipment(t *testing.T) *Equipment {
	t.Helper()
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)
	return NewEquipment(cat)
}

func TestAbilities_nothingEquipped(t *testing.T) {
	assert.Empty(t, newTestEquipment(t).Abilities())
}

func TestAbilities_equippedGrants(t *testing.T) {
	eq := newTestEquipment(t)
	censer := granting(t, "censer", rules.SlotHold, 3)
	eq.Equip(rules.SlotHold, censer)

	assert.Equal(t, map[string]Grant{"heal": {Instance: censer, Power: 3}}, eq.Abilities())
}

// two items granting one ability: the stronger one is what you cast with
func TestAbilities_highestPowerWins(t *testing.T) {
	eq := newTestEquipment(t)
	eq.Equip(rules.SlotNeck, granting(t, "charm", rules.SlotNeck, 2))
	censer := granting(t, "censer", rules.SlotHold, 8)
	eq.Equip(rules.SlotHold, censer)

	assert.Equal(t, Grant{Instance: censer, Power: 8}, eq.Abilities()["heal"])
}

// Review Focus 4: a broken censer grants nothing, even if it was the better one
func TestAbilities_brokenGrantsNothing(t *testing.T) {
	eq := newTestEquipment(t)
	charm := granting(t, "charm", rules.SlotNeck, 2)
	eq.Equip(rules.SlotNeck, charm)
	censer := granting(t, "censer", rules.SlotHold, 8)
	censer.Durability = 0
	require.True(t, censer.Broken())
	eq.Equip(rules.SlotHold, censer)

	assert.Equal(t, Grant{Instance: charm, Power: 2}, eq.Abilities()["heal"])

	eq.Unequip(rules.SlotNeck)
	assert.Empty(t, eq.Abilities(), "only the broken one left")
}
```

Append to `loader/abilities_test.go` (add `"github.com/watchmud/watchmud/rules"` to its imports):

```go
func TestObjectAbilities(t *testing.T) {
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)
	require.NoError(t, cat.SetAbilities([]*rules.Ability{{Id: "heal", Name: "heal", Target: rules.TargetFriend}}))

	got, err := objectAbilities("hollowfield", objectEntry{Id: "censer", Abilities: []string{"heal"}}, cat)
	require.NoError(t, err)
	assert.Equal(t, []string{"heal"}, got)

	_, err = objectAbilities("hollowfield", objectEntry{Id: "censer", Abilities: []string{"hael"}}, cat)
	assert.ErrorContains(t, err, "hollowfield/censer")
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./object ./loader -run 'Abilit'`
Expected: build failure -- `d.Abilities undefined`, `undefined: Grant`, `undefined: objectAbilities`.

- [ ] **Step 3: Implement**

`object/definition.go`, in `Definition` after `RoleWeights`:

```go
	// Abilities are what this lets its wearer cast while equipped and
	// unbroken, by rules.Ability id. Assigned by the loader, which refuses
	// an id the catalog doesn't define.
	Abilities []string
```

`object/equipment.go`, at the end:

```go
// A Grant is the item an ability comes from, and the power it is cast at.
type Grant struct {
	Instance *Instance
	Power    int
}

// Abilities is what the equipped, unbroken gear lets the wearer cast, keyed
// on ability id. Where two items grant the same one, the stronger is what you
// cast with; on a tie, the first in slot order. Recomputed every call, like
// role and power: nothing records what a player can do.
//
// A map, since only the listing cares about order, and that comes from the
// catalog -- see world.handleAbilities.
func (eq *Equipment) Abilities() map[string]Grant {
	grants := make(map[string]Grant)
	for _, inst := range eq.All() {
		if inst.Broken() {
			continue
		}
		for _, id := range inst.Definition.Abilities {
			if g, have := grants[id]; !have || inst.Power > g.Power {
				grants[id] = Grant{Instance: inst, Power: inst.Power}
			}
		}
	}
	return grants
}
```

`loader/objectfile.go`, in `objectEntry` after `Roles`:

```go
	// Abilities this grants while equipped, by rules.Ability id: ["heal"].
	Abilities []string `json:"abilities"`
```

`loader/content.go`, in `loadObjectDefinitions` after `d.RoleWeights = obj.Roles`:

```go
			if d.Abilities, err = objectAbilities(zonename, obj, c.Catalog); err != nil {
				return err
			}
```

and a helper at the end of `loader/content.go`:

```go
// objectAbilities checks what an object grants against the catalog: an
// unknown id is a typo, and ignoring it would leave a builder wondering why
// their censer doesn't heal.
func objectAbilities(zoneName string, obj objectEntry, cat *rules.Catalog) ([]string, error) {
	for _, id := range obj.Abilities {
		if _, known := cat.Abilities[id]; !known {
			return nil, fmt.Errorf("object %s/%s: unknown ability %q", zoneName, obj.Id, id)
		}
	}
	return obj.Abilities, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./object ./loader`
Expected: PASS

- [ ] **Step 5: `make check`, then commit**

```bash
make check
git add object/definition.go object/equipment.go object/abilities_test.go loader/objectfile.go loader/content.go loader/abilities_test.go
git commit -m "abilities: equipped, unbroken gear grants them, strongest wins

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Mana and cooldowns on the player, mana regen

**Files:**
- Modify: `player/player.go`, `player/record.go`, `mongostore/document.go`, `mongostore/document_test.go`, `rules/regen.go`, `world/regen.go`, `world/regen_test.go`
- Create: `player/mana_test.go`

**Interfaces:**
- Consumes: `rules.MaxMana` (Task 1).
- Produces: `(*Player).CurrentMana() int`, `MaxMana() int`, `SpendMana(n int) bool`,
  `RestoreMana(n int)`, `ReadyAt(abilityId string) time.Time`,
  `StartCooldown(abilityId string, until time.Time)`;
  `player.Record.CurMana *int`; `rules.ManaRegenAmount(maxMana int) int`.

- [ ] **Step 1: Write the failing tests** -- `player/mana_test.go`

```go
package player

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/rules"
)

func TestMana_startsFull(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "wren", nil)
	assert.Equal(t, rules.MaxMana, p.MaxMana())
	assert.Equal(t, rules.MaxMana, p.CurrentMana())
}

func TestMana_spendAndRestore(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "wren", nil)
	assert.True(t, p.SpendMana(30))
	assert.Equal(t, 70, p.CurrentMana())
	assert.False(t, p.SpendMana(71), "can't spend what you haven't got")
	assert.Equal(t, 70, p.CurrentMana(), "and a refused spend costs nothing")
	assert.False(t, p.SpendMana(-5))

	p.RestoreMana(50)
	assert.Equal(t, 100, p.CurrentMana(), "capped at max")
}

func TestMana_onTheRecord(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "wren", nil)
	p.SpendMana(45)
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)

	back, err := FromRecord(p.Record(), &Recorder{}, cat, nil)
	require.NoError(t, err)
	assert.Equal(t, 55, back.CurrentMana())
}

// a record from before mana existed: full, not empty
func TestMana_oldRecordIsFull(t *testing.T) {
	rec := NewTestPlayer(uuid.New(), "wren", nil).Record()
	rec.CurMana = nil
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)

	back, err := FromRecord(rec, &Recorder{}, cat, nil)
	require.NoError(t, err)
	assert.Equal(t, rules.MaxMana, back.CurrentMana())
}

// a record claiming more than max (max came down in content) is clamped
func TestMana_recordClamped(t *testing.T) {
	rec := NewTestPlayer(uuid.New(), "wren", nil).Record()
	tooMuch, negative := 150, -3
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)

	rec.CurMana = &tooMuch
	back, err := FromRecord(rec, &Recorder{}, cat, nil)
	require.NoError(t, err)
	assert.Equal(t, rules.MaxMana, back.CurrentMana())

	rec.CurMana = &negative
	back, err = FromRecord(rec, &Recorder{}, cat, nil)
	require.NoError(t, err)
	assert.Zero(t, back.CurrentMana())
}

func TestCooldown(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "wren", nil)
	assert.True(t, p.ReadyAt("heal").IsZero(), "never cast: ready")

	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	p.StartCooldown("heal", at)
	assert.Equal(t, at, p.ReadyAt("heal"))
	assert.True(t, p.ReadyAt("smite").IsZero(), "one ability's cooldown is its own")
}
```

In `world/regen_test.go`, add:

```go
// Mana comes back in a fight -- the healer is busy the whole fight -- while
// health still doesn't.
func (s *regenSuite) TestManaEvenWhileFighting() {
	target, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)
	s.Require().NoError(s.w.fightLedger.Fight(s.p, target))
	s.p.TakeMeleeDamage(50)
	s.Require().True(s.p.SpendMana(60))

	s.w.Regenerate()

	s.Assert().Equal(45, s.p.CurrentMana(), "5% of 100")
	s.Assert().Equal(50, s.p.CurrentHealth(), "still no health mid-fight")
}
```

In `mongostore/document_test.go`, in `testRecord()`, add a line after `MaxHealth: 100,`:

```go
		CurMana:      intp(55),
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./player ./world ./mongostore -run 'Mana|Cooldown|PlayerDoc'`
Expected: build failure -- `p.MaxMana undefined`, `unknown field CurMana`.

- [ ] **Step 3: Implement**

`player/player.go`: add `"time"` to the imports; in the struct, after `maxHealth int`:

```go
	curMana int
	maxMana int
	// readyAt is when each ability (by id) can be cast again. In memory
	// only: a quit resets it, and logging back in takes longer than any
	// cooldown.
	readyAt map[string]time.Time
```

in `New`, after `maxHealth: 100,`:

```go
		curMana:      rules.MaxMana,
		maxMana:      rules.MaxMana,
```

and methods, after `Revive`:

```go
func (p *Player) CurrentMana() int { return p.curMana }
func (p *Player) MaxMana() int     { return p.maxMana }

// SpendMana takes n if there is that much, and says whether it did.
func (p *Player) SpendMana(n int) bool {
	if n < 0 || n > p.curMana {
		return false
	}
	p.curMana -= n
	return true
}

func (p *Player) RestoreMana(n int) {
	p.curMana = min(p.curMana+max(n, 0), p.maxMana)
}

// ReadyAt is when this ability can next be cast; the zero time if it never
// has been.
func (p *Player) ReadyAt(abilityId string) time.Time {
	return p.readyAt[abilityId]
}

func (p *Player) StartCooldown(abilityId string, until time.Time) {
	if p.readyAt == nil {
		p.readyAt = make(map[string]time.Time)
	}
	p.readyAt[abilityId] = until
}
```

In `FromRecord`, after `p.maxHealth = rec.MaxHealth`:

```go
	if rec.CurMana != nil {
		// absent is a record from before mana, which leaves New's full pool
		p.curMana = min(max(*rec.CurMana, 0), p.maxMana)
	}
```

In `Record()`, add `CurMana: &mana,` to the literal, with `mana := p.curMana` above the `return`.

`player/record.go`, in `Record` after `CurHealth, MaxHealth int`:

```go
	// CurMana is a pointer for the Durability reason: a record from before
	// mana existed reads as full, not empty. Max isn't saved; it's
	// rules.MaxMana for everyone.
	CurMana *int
```

`mongostore/document.go`: in `playerDoc` after `MaxHealth`:

```go
	CurMana      *int   `bson:"cur_mana,omitempty"`
```

and `CurMana: r.CurMana,` / `CurMana: d.CurMana,` beside the two `MaxHealth:` lines in the converters.

`rules/regen.go`, at the end:

```go
// ManaRegenPercent is how much of their max mana everyone gets back each
// regen pulse -- fighting or not, unlike health. A placeholder: LEVELS.md,
// "Tuning placeholders".
const ManaRegenPercent = 5

// ManaRegenAmount is one pulse's worth, at least a point.
func ManaRegenAmount(maxMana int) int {
	return max(1, maxMana*ManaRegenPercent/100)
}
```

`world/regen.go`, the player loop becomes:

```go
	for p := range w.Players() {
		// mana before the fight check: a healer gets it back mid-fight
		p.RestoreMana(rules.ManaRegenAmount(p.MaxMana()))
		if p.Dead() || w.fightLedger.InFight(p) {
			continue
		}
		p.RestoreHealth(rules.RegenAmount(p.MaxHealth()))
	}
```

and the doc comment gains: "Mana is the exception: it comes back fighting or not."

- [ ] **Step 4: Run the tests**

Run: `go test ./player ./world ./mongostore ./rules`
Expected: PASS (the mongo-backed store tests skip without `WATCHMUD_TEST_MONGO_URI`; run `make db-up && make test-db` if you want them).

- [ ] **Step 5: `make check`, then commit**

```bash
make check
git add player/ mongostore/document.go mongostore/document_test.go rules/regen.go world/regen.go world/regen_test.go
git commit -m "abilities: mana on the player and the record; it regens even mid-fight

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Mana in the prompt, and the bots

**Files:**
- Modify: `event/events.go`, `server/gameserver.go`, `server/gameserver_test.go`, `telnet/render.go`, `telnet/ansi_test.go`, `bot/client.go`, `bot/client_test.go`, `bot/smoke.go`

**Interfaces:**
- Consumes: `CurrentMana()`/`MaxMana()` (Task 3).
- Produces: `event.Prompt{CurrentHealth, MaxHealth, CurrentMana, MaxMana int}`;
  wire format `<97/100hp 80/100m> ` when `MaxMana > 0`.

- [ ] **Step 1: Write the failing tests**

`telnet/ansi_test.go`, add:

```go
// mana follows health, uncolored -- health is the warning, mana is a number.
// A prompt with no mana pool says nothing about mana.
func TestPrompt_mana(t *testing.T) {
	assert.Equal(t, "<"+green+"100/100"+reset+"hp 80/100m> ",
		render(event.Prompt{CurrentHealth: 100, MaxHealth: 100, CurrentMana: 80, MaxMana: 100}, "testdood"))
	// Review Focus 5: empty is still shown
	assert.Equal(t, "<100/100hp 0/100m> ",
		plain(render(event.Prompt{CurrentHealth: 100, MaxHealth: 100, CurrentMana: 0, MaxMana: 100}, "testdood")))
	assert.Equal(t, "<100/100hp> ", plain(render(event.Prompt{CurrentHealth: 100, MaxHealth: 100}, "testdood")))
}
```

`server/gameserver_test.go`, in `TestPrompt_reachesPlayersInTheWorld`, change the assertion to:

```go
	assert.Equal(t, event.Prompt{CurrentHealth: 70, MaxHealth: 100, CurrentMana: 100, MaxMana: 100}, c.sent[len(c.sent)-1])
```

`bot/client_test.go` -- read the file first and copy the shape of its existing
`ReadChunk` test; add one that feeds `"Ok.\r\n<95/100hp 80/100m> "` and asserts
`Health == 95`, `MaxHealth == 100`, `Text == "Ok.\r\n"`.

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./telnet ./server ./bot -run 'Prompt|Chunk'`
Expected: build failure on `CurrentMana`, then (after Step 3's event change) render and bot failures.

- [ ] **Step 3: Implement**

`event/events.go`, `Prompt`:

```go
type Prompt struct {
	CurrentHealth int
	MaxHealth     int
	CurrentMana   int
	MaxMana       int // zero: no mana pool, and the prompt doesn't mention one
}
```

`server/gameserver.go`, in `prompt()`:

```go
		p.Send(event.Prompt{
			CurrentHealth: p.CurrentHealth(), MaxHealth: p.MaxHealth(),
			CurrentMana: p.CurrentMana(), MaxMana: p.MaxMana(),
		})
```

`telnet/render.go`, the `event.Prompt` case:

```go
	case event.Prompt:
		hp := fmt.Sprintf("%d/%d", m.CurrentHealth, m.MaxHealth)
		s := "<" + paint(healthColor(m.CurrentHealth, m.MaxHealth), hp) + "hp"
		if m.MaxMana > 0 {
			s += fmt.Sprintf(" %d/%dm", m.CurrentMana, m.MaxMana)
		}
		return s + "> "
```

`bot/client.go`:

```go
// the mana half is optional: the bot only reads health
var promptRe = regexp.MustCompile(`<(\d+)/(\d+)hp(?: \d+/\d+m)?> `)
```

`bot/smoke.go`:

```go
// prompt is the server's prompt, which ends every reply.
const prompt = `<\d+/\d+hp(?: \d+/\d+m)?> `

// lineStart matches the start of a line -- or just after a prompt, since the
// bot doesn't echo what it types and the reply lands on the prompt's line.
const lineStart = `(?m)^(?:` + prompt + `)?`
```

and the `get all from corpse` expectation becomes:

```go
	m, err := c.Expect(`(?s)((?:You get |There's nothing in there\.).*?)`+prompt, stepTimeout)
```

- [ ] **Step 4: Run the tests, including the bots against the real world**

Run: `go test ./telnet ./server ./bot`
Expected: PASS. `bot/smoke_test.go` and `adventurer_game_test.go` run the real server in-process, so they prove the bots read the new prompt.

- [ ] **Step 5: `make check`, then commit**

```bash
make check
git add event/events.go server/ telnet/render.go telnet/ansi_test.go bot/
git commit -m "abilities: mana in the prompt; the bots read past it

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: `cast heal`

**Files:**
- Create: `world/abilities.go`, `world/h_cast.go`, `world/h_cast_test.go`
- Modify: `command/commands.go`, `event/result.go`, `event/events.go`, `world/world.go`, `world/handlers.go`, `telnet/parse.go`, `telnet/render.go`, `telnet/resultcode.go`, `telnet/help.go`, `telnet/render_test.go`, `testcontent/world/wrathrock/objects.json`

**Interfaces:**
- Consumes: everything above.
- Produces: `command.Cast{Ability, Target string}` (verb `cast`);
  codes `event.UnknownAbility`, `NotGranted`, `NotReady`, `NotEnoughMana`;
  `event.Healed{Actor, Target string; Amount int}`;
  `World.now func() time.Time`; `world.effects map[string]effect`.

- [ ] **Step 1: The test fixture** -- `testcontent/world/wrathrock/objects.json`, `healers_censer` gains a key:

```json
    "roles": { "healer": 3, "tank": 1 },
    "abilities": [ "heal" ]
```

- [ ] **Step 2: Write the failing tests** -- `world/h_cast_test.go`

```go
package world

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

type handleCastSuite struct {
	worldTestSuite
	clock time.Time
	bob   *player.Player
	bobR  *player.Recorder
}

func TestHandleCastSuite(t *testing.T) {
	suite.Run(t, new(handleCastSuite))
}

func (s *handleCastSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.clock = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	s.w.now = func() time.Time { return s.clock }
	s.bobR = &player.Recorder{}
	s.bob = player.NewTestPlayer(uuid.New(), "bob", s.bobR)
	s.w.PlacePlayer(s.bob, s.w.StartRoom)
}

// hold a censer granting heal, at this power
func (s *handleCastSuite) holdCenser(power int) *object.Instance {
	s.T().Helper()
	d := object.NewTestDefinition(s.T(), "censer", rules.SlotHold, rules.ObjectCategoryOther, rules.ArmorTypeNone)
	d.Abilities = []string{"heal"}
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = power
	s.Require().NoError(s.p.Inventory().Add(inst))
	s.p.Equipment().Equip(rules.SlotHold, inst)
	return inst
}

func (s *handleCastSuite) cast(ability, target string) {
	s.T().Helper()
	s.r.Sent, s.bobR.Sent = nil, nil
	cmd := command.Cast{Ability: ability, Target: target}
	s.w.handleCast(s.handlerParameter(cmd), cmd)
}

func (s *handleCastSuite) failed() event.ResultCode {
	s.T().Helper()
	return sent[event.Failed](s.T(), s.r, 0).Code
}

func (s *handleCastSuite) TestHealSelf() {
	s.holdCenser(1)
	s.p.TakeMeleeDamage(50)

	s.cast("heal", "")

	s.Assert().Equal(62, s.p.CurrentHealth(), "10 + 2 per power")
	s.Assert().Equal(80, s.p.CurrentMana())
	s.Assert().Equal(event.Healed{Actor: "testdood", Target: "testdood", Amount: 12}, sent[event.Healed](s.T(), s.r, 0))
}

func (s *handleCastSuite) TestHealOther() {
	s.holdCenser(3)
	s.bob.TakeMeleeDamage(50)

	s.cast("heal", "bob")

	s.Assert().Equal(66, s.bob.CurrentHealth())
	want := event.Healed{Actor: "testdood", Target: "bob", Amount: 16}
	s.Assert().Equal(want, sent[event.Healed](s.T(), s.r, 0))
	s.Assert().Equal(want, sent[event.Healed](s.T(), s.bobR, 0), "the room sees it")
}

// Review Focus 1 and 2: case, and yourself by name
func (s *handleCastSuite) TestCaseAndOwnName() {
	s.holdCenser(1)
	s.bob.TakeMeleeDamage(50)
	s.cast("HEAL", "Bob")
	s.Assert().Equal(62, s.bob.CurrentHealth())

	s.clock = s.clock.Add(time.Minute)
	s.p.TakeMeleeDamage(50)
	s.cast("heal", "testdood")
	s.Assert().Equal(62, s.p.CurrentHealth())
}

// a wasted heal is still a heal: it spends, and says it was wasted
func (s *handleCastSuite) TestUnhurtStillSpends() {
	s.holdCenser(1)

	s.cast("heal", "bob")

	s.Assert().Equal(80, s.p.CurrentMana())
	s.Assert().Zero(sent[event.Healed](s.T(), s.r, 0).Amount)
	s.cast("heal", "bob")
	s.Assert().Equal(event.NotReady, s.failed(), "and the cooldown started")
}

func (s *handleCastSuite) TestMidFight() {
	s.holdCenser(1)
	target, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)
	s.Require().NoError(s.w.fightLedger.Fight(s.p, target))
	s.p.TakeMeleeDamage(50)

	s.cast("heal", "")

	s.Assert().Equal(62, s.p.CurrentHealth())
}

func (s *handleCastSuite) TestCooldown() {
	s.holdCenser(1)
	s.cast("heal", "")

	s.clock = s.clock.Add(9 * time.Second)
	s.cast("heal", "")
	s.Assert().Equal(event.NotReady, s.failed())
	s.Assert().Equal(80, s.p.CurrentMana(), "refused casts cost nothing")

	s.clock = s.clock.Add(time.Second)
	s.cast("heal", "")
	s.Assert().Equal(60, s.p.CurrentMana(), "ready again at exactly the cooldown")
}

func (s *handleCastSuite) TestRefusals() {
	s.cast("", "")
	s.Assert().Equal(event.NoTarget, s.failed())

	s.cast("fireball", "")
	s.Assert().Equal(event.UnknownAbility, s.failed())

	s.cast("heal", "")
	s.Assert().Equal(event.NotGranted, s.failed(), "nothing equipped")

	s.holdCenser(1)
	s.cast("heal", "nobody")
	s.Assert().Equal(event.TargetNotFound, s.failed())

	s.Require().True(s.p.SpendMana(85))
	s.cast("heal", "")
	s.Assert().Equal(event.NotEnoughMana, s.failed())
	s.Assert().Equal(15, s.p.CurrentMana())
}

// a censer in your pack does nothing
func (s *handleCastSuite) TestCarriedIsNotEquipped() {
	censer := s.holdCenser(1)
	s.p.Equipment().Unequip(rules.SlotHold)
	_, carried := s.p.Inventory().Get(censer.Id)
	s.Require().True(carried)

	s.cast("heal", "")
	s.Assert().Equal(event.NotGranted, s.failed())
}

// Review Focus 3: no heals through walls, and a miss costs nothing
func (s *handleCastSuite) TestOtherRoom() {
	s.holdCenser(1)
	smithy, found := s.w.findRoomById("wrathrock", "smithy")
	s.Require().True(found)
	s.w.movePlayerMagically(s.bob, smithy)

	s.cast("heal", "bob")

	s.Assert().Equal(event.TargetNotFound, s.failed())
	s.Assert().Equal(100, s.p.CurrentMana())
	s.Assert().True(s.p.ReadyAt("heal").IsZero())
}

// every ability in the catalog has an effect, or the world won't build
func (s *handleCastSuite) TestEveryAbilityHasAnEffect() {
	s.Assert().NoError(checkEffects(s.w.content.Catalog))
	cat, err := rules.NewTestCatalog()
	s.Require().NoError(err)
	s.Require().NoError(cat.SetAbilities([]*rules.Ability{{Id: "levitate", Name: "levitate", Target: rules.TargetSelf}}))
	s.Assert().ErrorContains(checkEffects(cat), "levitate")
}
```

`telnet/render_test.go`, add to `commandCases` (find a spot near the `recall` case):

```go
	{
		name: "cast heal on another",
		setup: func(w *world.World, p *player.Player, o *player.Player) {
			holdTestCenser(p)
			o.TakeMeleeDamage(50)
		},
		input:     "cast heal otherdood",
		want:      "You heal otherdood. (+12)\n",
		wantOther: "testdood heals you. (+12)\n",
	},
	{
		name: "c is cast, and heal on yourself",
		setup: func(w *world.World, p *player.Player, _ *player.Player) {
			holdTestCenser(p)
			p.TakeMeleeDamage(50)
		},
		input:     "c heal",
		want:      "You heal yourself. (+12)\n",
		wantOther: "testdood casts a heal.\n",
	},
	{
		name:      "healing the unhurt is wasted",
		setup:     func(_ *world.World, p *player.Player, _ *player.Player) { holdTestCenser(p) },
		input:     "cast heal otherdood",
		want:      "You heal otherdood, but otherdood wasn't hurt.\n",
		wantOther: "testdood heals you, but you weren't hurt.\n",
	},
	{
		name:  "cast without the gear",
		input: "cast heal",
		want:  "Nothing you're wearing lets you cast that.\n",
	},
	{
		name:  "cast nothing",
		input: "cast",
		want:  "Cast what?\n",
	},
```

and the helper next to `testKnife()` (power 1, so the numbers match `TestHealSelf`):

```go
// holdTestCenser equips something granting heal at power 1
func holdTestCenser(p *player.Player) {
	d := object.NewDefinition("censer", "censer", "wrathrock", rules.ObjectCategoryOther,
		nil, "a censer", "A censer is here.", rules.SlotHold, rules.ArmorTypeNone, nil)
	d.Abilities = []string{"heal"}
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = 1
	_ = p.Inventory().Add(inst)
	p.Equipment().Equip(rules.SlotHold, inst)
}
```

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./world ./telnet -run 'Cast|CommandRendering'`
Expected: build failure -- `undefined: command.Cast`, `s.w.now undefined`, `undefined: checkEffects`.

- [ ] **Step 4: The vocabulary**

`command/commands.go`, after `Role`:

```go
// Cast uses an ability the player's gear grants: cast heal, cast heal bob.
// Ability is the first word as typed; Target is the rest, raw.
type Cast struct {
	Ability string
	Target  string
}

func (Cast) Verb() string { return "cast" }
```

`event/result.go`, a new group after the shop codes:

```go
	// abilities
	UnknownAbility ResultCode = "UNKNOWN_ABILITY"
	NotGranted     ResultCode = "NOT_GRANTED"
	NotReady       ResultCode = "NOT_READY"
	NotEnoughMana  ResultCode = "NOT_ENOUGH_MANA"
```

`event/events.go`, after `Restored`:

```go
// Healed goes to the whole room. Amount is what was actually restored, which
// is 0 when the target wasn't hurt: the cast still happened.
type Healed struct {
	Actor  string
	Target string
	Amount int
}
```

- [ ] **Step 5: The world**

`world/world.go`: add `"time"` to the imports; in the `World` struct, after `scripts`:

```go
	// now is the clock cooldowns read; tests replace it rather than sleep.
	now func() time.Time
```

in `New`'s literal, `now: time.Now,`; and after the scripts runtime is built:

```go
	if err := checkEffects(c.Catalog); err != nil {
		return nil, fmt.Errorf("building world: %w", err)
	}
```

`world/abilities.go`:

```go
package world

import (
	"fmt"

	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

// cast is one use of an ability, already checked and paid for.
type cast struct {
	caster  *player.Player
	target  *player.Player // for self and friend; foe will add a mob
	ability *rules.Ability
	power   int // of the item granting it
}

// An effect is what an ability does. Content says how much; this says what.
// It can't refuse: by the time it runs the cast is paid for.
type effect func(w *World, c cast)

// effects is every ability the engine knows how to do, by rules.Ability id.
var effects = map[string]effect{
	"heal": healEffect,
}

// checkEffects refuses a catalog naming an ability the engine can't do --
// the other half of the loader refusing an object granting one the catalog
// doesn't define.
func checkEffects(cat *rules.Catalog) error {
	for _, a := range cat.AbilityList() {
		if _, ok := effects[a.Id]; !ok {
			return fmt.Errorf("ability %q has no effect in world/abilities.go", a.Id)
		}
	}
	return nil
}

// healEffect gives the target health back, more for a stronger item. Fine
// mid-fight -- that's what it's for -- and it neither starts nor joins one.
func healEffect(w *World, c cast) {
	before := c.target.CurrentHealth()
	c.target.RestoreHealth(c.ability.Amount.For(c.power))
	w.playerRoom(c.caster).Send(event.Healed{
		Actor:  c.caster.Name(),
		Target: c.target.Name(),
		Amount: c.target.CurrentHealth() - before,
	})
}
```

`world/h_cast.go`:

```go
package world

import (
	"strings"

	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

// handleCast uses an ability the player's equipped gear grants. Every
// refusal comes before anything is spent; once the checks pass the cast
// happens and is paid for, even if it turns out to do nothing (healing
// someone who isn't hurt).
func (w *World) handleCast(msg *gameserver.HandlerParameter, cmd command.Cast) {
	p := msg.Player
	if cmd.Ability == "" {
		msg.Fail(event.NoTarget)
		return
	}
	a, known := w.content.Catalog.Abilities[strings.ToLower(cmd.Ability)]
	if !known {
		msg.Fail(event.UnknownAbility)
		return
	}
	grant, granted := p.Equipment().Abilities()[a.Id]
	if !granted {
		msg.Fail(event.NotGranted)
		return
	}
	now := w.now()
	if now.Before(p.ReadyAt(a.Id)) {
		msg.Fail(event.NotReady)
		return
	}
	target, code := w.castTarget(p, a, cmd.Target)
	if code != "" {
		msg.Fail(code)
		return
	}
	if !p.SpendMana(a.Mana) {
		msg.Fail(event.NotEnoughMana)
		return
	}
	p.StartCooldown(a.Id, now.Add(a.Cooldown))
	effects[a.Id](w, cast{caster: p, target: target, ability: a, power: grant.Power})
}

// castTarget resolves the target by the ability's kind. Only the kinds an
// ability uses so far are here; foe arrives with the first offensive one.
func (w *World) castTarget(caster *player.Player, a *rules.Ability, target string) (*player.Player, event.ResultCode) {
	switch a.Target {
	case rules.TargetSelf:
		return caster, ""
	case rules.TargetFriend:
		if target == "" {
			return caster, ""
		}
		if p, found := w.playerRoom(caster).FindPlayer(target); found {
			return p, ""
		}
		return nil, event.TargetNotFound
	}
	caster.Log().Error().Msgf("cast %s: target kind %q not handled yet", a.Id, a.Target)
	return nil, event.InternalError
}
```

`world/handlers.go`, in the switch (alphabetical-ish, near `command.Consider`):

```go
	case command.Cast:
		w.handleCast(msg, cmd)
```

- [ ] **Step 6: The telnet half**

`telnet/parse.go`, near `"role"`:

```go
	case "cast", "c":
		// the first word is the ability; the rest is the target, raw
		if len(tokens) < 2 {
			return command.Cast{}, nil
		}
		return command.Cast{Ability: tokens[1], Target: strings.Join(tokens[2:], " ")}, nil
```

`telnet/resultcode.go`, in `failureByVerb`:

```go
	"cast/NO_TARGET":        "Cast what?",
	"cast/TARGET_NOT_FOUND": "There's no one here by that name.",
```

and in `failureByCode`, a new group after the shop lines:

```go
	// abilities
	"UNKNOWN_ABILITY": "There's no such spell.",
	"NOT_GRANTED":     "Nothing you're wearing lets you cast that.",
	"NOT_READY":       "You can't cast that again yet.",
	"NOT_ENOUGH_MANA": "You don't have enough mana.",
```

`telnet/render.go`, after the `event.Restored` case:

```go
	case event.Healed:
		return renderHealed(m, self)
```

and the function (near the other `render*` helpers):

```go
// renderHealed words one heal for whoever is looking. A heal that found
// nothing to mend still happened -- it cost the caster -- so it says so.
func renderHealed(m event.Healed, self string) string {
	wasted := m.Amount == 0
	switch {
	case m.Actor == self && m.Target == self:
		if wasted {
			return "You heal yourself, but you weren't hurt.\n"
		}
		return fmt.Sprintf("You heal yourself. (+%d)\n", m.Amount)
	case m.Actor == self:
		if wasted {
			return fmt.Sprintf("You heal %s, but %s wasn't hurt.\n", m.Target, m.Target)
		}
		return fmt.Sprintf("You heal %s. (+%d)\n", m.Target, m.Amount)
	case m.Target == self:
		if wasted {
			return m.Actor + " heals you, but you weren't hurt.\n"
		}
		return fmt.Sprintf("%s heals you. (+%d)\n", m.Actor, m.Amount)
	case m.Actor == m.Target:
		return m.Actor + " casts a heal.\n"
	}
	return m.Actor + " heals " + m.Target + ".\n"
}
```

`telnet/help.go`, in the `"Fighting"` group:

```go
		{"cast <ability> [player]", "use what your gear grants: cast heal, cast heal bob (c)", []string{"cast", "c"}},
```

- [ ] **Step 7: Run the tests**

Run: `go test ./world ./telnet`
Expected: PASS

- [ ] **Step 8: `make check`, then commit**

```bash
make check
git add command/ event/ world/ telnet/ testcontent/world/wrathrock/objects.json
git commit -m "abilities: cast heal -- mana, cooldown, yourself or a player here

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: `abilities`

**Files:**
- Create: `world/h_abilities.go`, `world/h_abilities_test.go`
- Modify: `command/commands.go`, `event/events.go`, `world/handlers.go`, `telnet/parse.go`, `telnet/render.go`, `telnet/help.go`, `telnet/render_test.go`

**Interfaces:**
- Consumes: `Catalog.AbilityList()`, `Equipment.Abilities()`, `ReadyAt`, `World.now`.
- Produces: `command.Abilities{}` (verb `abilities`);
  `event.Abilities{Granted []event.GrantedAbility}`,
  `event.GrantedAbility{Name string; Mana int; Cooldown, ReadyIn time.Duration; Item string; Power int}`.

- [ ] **Step 1: Write the failing tests** -- `world/h_abilities_test.go`

```go
package world

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

type handleAbilitiesSuite struct {
	worldTestSuite
	clock time.Time
}

func TestHandleAbilitiesSuite(t *testing.T) {
	suite.Run(t, new(handleAbilitiesSuite))
}

func (s *handleAbilitiesSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.clock = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	s.w.now = func() time.Time { return s.clock }
}

func (s *handleAbilitiesSuite) list() event.Abilities {
	s.T().Helper()
	s.r.Sent = nil
	s.w.handleAbilities(s.handlerParameter(command.Abilities{}), command.Abilities{})
	return sent[event.Abilities](s.T(), s.r, 0)
}

func (s *handleAbilitiesSuite) TestNothing() {
	s.Assert().Empty(s.list().Granted)
}

func (s *handleAbilitiesSuite) TestGrantedAndCooling() {
	d := object.NewTestDefinition(s.T(), "censer", rules.SlotHold, rules.ObjectCategoryOther, rules.ArmorTypeNone)
	d.Abilities = []string{"heal"}
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = 3
	s.Require().NoError(s.p.Inventory().Add(inst))
	s.p.Equipment().Equip(rules.SlotHold, inst)

	want := event.GrantedAbility{Name: "heal", Mana: 20, Cooldown: 10 * time.Second, Item: "censer", Power: 3}
	s.Assert().Equal([]event.GrantedAbility{want}, s.list().Granted)

	s.p.StartCooldown("heal", s.clock.Add(4*time.Second))
	want.ReadyIn = 4 * time.Second
	s.Assert().Equal([]event.GrantedAbility{want}, s.list().Granted)
}
```

`telnet/render_test.go`, add to `commandCases`:

```go
	{
		name:  "abilities with nothing",
		input: "abilities",
		want:  "Nothing you're wearing grants any abilities.\n",
	},
	{
		name:  "abilities lists what the gear grants",
		setup: func(_ *world.World, p *player.Player, _ *player.Player) { holdTestCenser(p) },
		input: "abilities",
		want:  "heal         20 mana  10s cooldown  a censer (power 1)  ready\n",
	},
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./world ./telnet -run 'Abilities|CommandRendering'`
Expected: build failure -- `undefined: command.Abilities`.

- [ ] **Step 3: Implement**

`command/commands.go`, after `Cast`:

```go
// Abilities lists what the player's gear lets them cast, right now.
type Abilities struct{}

func (Abilities) Verb() string { return "abilities" }
```

`event/events.go` (add `"time"` to its imports -- stdlib is allowed in this leaf package):

```go
// Abilities is what the player's equipped gear grants, in catalog order.
type Abilities struct {
	Granted []GrantedAbility
}

type GrantedAbility struct {
	Name     string
	Mana     int
	Cooldown time.Duration
	Item     string // short description of the item granting it
	Power    int
	ReadyIn  time.Duration // zero: ready
}
```

`world/h_abilities.go`:

```go
package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

// handleAbilities lists what the player's gear lets them cast, in the order
// the catalog declares them, so the list doesn't reshuffle between asks.
func (w *World) handleAbilities(msg *gameserver.HandlerParameter, _ command.Abilities) {
	p := msg.Player
	grants := p.Equipment().Abilities()
	now := w.now()
	var listed []event.GrantedAbility
	for _, a := range w.content.Catalog.AbilityList() {
		g, granted := grants[a.Id]
		if !granted {
			continue
		}
		listed = append(listed, event.GrantedAbility{
			Name:     a.Name,
			Mana:     a.Mana,
			Cooldown: a.Cooldown,
			Item:     g.Instance.Definition.ShortDescription,
			Power:    g.Power,
			ReadyIn:  max(p.ReadyAt(a.Id).Sub(now), 0),
		})
	}
	p.Send(event.Abilities{Granted: listed})
}
```

`world/handlers.go`:

```go
	case command.Abilities:
		w.handleAbilities(msg, cmd)
```

`telnet/parse.go`, beside `cast`:

```go
	case "abilities", "abil":
		return command.Abilities{}, nil
```

`telnet/render.go`, after the `event.Healed` case:

```go
	case event.Abilities:
		return renderAbilities(m)
```

and:

```go
func renderAbilities(m event.Abilities) string {
	if len(m.Granted) == 0 {
		return "Nothing you're wearing grants any abilities.\n"
	}
	var b strings.Builder
	for _, a := range m.Granted {
		ready := "ready"
		if a.ReadyIn > 0 {
			ready = fmt.Sprintf("ready in %ds", int(a.ReadyIn.Round(time.Second)/time.Second))
		}
		fmt.Fprintf(&b, "%-12s %2d mana  %s cooldown  %s (power %d)  %s\n",
			a.Name, a.Mana, a.Cooldown, a.Item, a.Power, ready)
	}
	return b.String()
}
```

(add `"time"` to render.go's imports if it isn't there.) A sub-second remainder rounds:
`ReadyIn` of 400ms shows "ready in 0s" -- if that reads badly in play, round up with
`(a.ReadyIn + time.Second - 1) / time.Second`; mention it at review.

`telnet/help.go`, in the `"You"` group after `role`:

```go
		{"abilities", "what your gear lets you cast", []string{"abilities"}},
```

- [ ] **Step 4: Run the tests**

Run: `go test ./world ./telnet`
Expected: PASS

- [ ] **Step 5: `make check`, then commit**

```bash
make check
git add command/ event/ world/ telnet/
git commit -m "abilities: the abilities command lists what your gear grants

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Content -- the censers and the charm heal

**Files:**
- Modify: `content/world/hollowfield/objects.json`, `content/world/sample/objects.json`, `content/world/barrow/objects.json`, `loader/abilities_test.go`

- [ ] **Step 1: Write the failing test** -- append to `loader/abilities_test.go`:

```go
// a newbie's way to a heal, and a barrow healer's better one
func TestLoadContent_healersHeal(t *testing.T) {
	c, err := LoadContent(os.DirFS("../content"))
	require.NoError(t, err)
	for _, ref := range [][2]string{
		{"hollowfield", "sprig_censer"},
		{"barrow", "bone_charm"},
	} {
		d, found := c.Zones[ref[0]].ObjectDefinitions[ref[1]]
		require.True(t, found, ref)
		assert.Contains(t, d.Abilities, "heal", ref)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./loader -run HealersHeal`
Expected: FAIL -- `"[]" does not contain "heal"`.

- [ ] **Step 3: The content**

Add `"abilities": [ "heal" ]` beside `"roles"` on:
- `sprig_censer` in `content/world/hollowfield/objects.json`
- `healers_censer` in `content/world/sample/objects.json`
- `bone_charm` in `content/world/barrow/objects.json`

e.g. sprig_censer ends:

```json
    "equipment_slot": "hold",
    "roles": {
      "healer": 3
    },
    "abilities": [
      "heal"
    ]
```

- [ ] **Step 4: Run the tests, the bots included**

Run: `go test ./loader ./bot`
Expected: PASS.

- [ ] **Step 5: Try it** -- `make run`, `telnet localhost 4000`, a wizard character: `load obj hollowfield sprig_censer` (or whatever `load`'s syntax is: `help` won't show it; read `world/h_wiz_load.go`), `hold censer`, `abilities`, `cast heal`; watch the prompt's mana drop and come back.

- [ ] **Step 6: `make check`, then commit**

```bash
make check
git add content/world/ loader/abilities_test.go
git commit -m "content: the censers and the bone charm heal

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Docs

**Files:**
- Modify: `CLAUDE.md`, `LEVELS.md`, `content/rules/README.md`, `site/index.html`

No tests; `make check` still has to pass (the site isn't checked).

- [ ] **Step 1: CLAUDE.md** -- a `### Abilities` section after "Lineage and Role", covering:
  gear grants abilities (equipped, unbroken; highest power wins; derived, never stored);
  `abilities.json` holds the numbers and `world/abilities.go` the effects, each half
  checked at startup; the cast's check order and "a cast that passes always spends";
  mana flat 100, saved as `*int`, regens even mid-fight; cooldowns in memory and read
  through `World.now`; the prompt's mana half only when `MaxMana > 0`; **adding an
  ability** = an entry in abilities.json, an effect in `effects`, a target kind if it
  needs a new one, its own event, and grants in objects.json. Also add `cast`/`abilities`
  nowhere special -- they follow "Adding a command" already.
- [ ] **Step 2: LEVELS.md** -- move "Abilities from gear" out of Tabled into Phases as
  done-for-heal (offensive/defensive/informative/recall still to come); add rows to
  "Tuning placeholders": heal cost/cooldown/amount (`abilities.json`), mana regen 5%
  (`rules.ManaRegenPercent`), max mana 100 (`rules.MaxMana`).
- [ ] **Step 3: content/rules/README.md** -- a `## Abilities` section: the file's
  fields, the `target` kinds, that an object grants with `"abilities"`, and that an
  ability with no Go effect fails startup.
- [ ] **Step 4: site/index.html** -- a short paragraph on `cast heal`, `abilities` and the
  mana in the prompt. **Commit it, but don't push it** until the release that ships
  abilities (the site publishes on any master push that touches it).
- [ ] **Step 5: commit**

```bash
make check
git add CLAUDE.md LEVELS.md content/rules/README.md site/index.html
git commit -m "docs: abilities from gear, and cast heal

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
