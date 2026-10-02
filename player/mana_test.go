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

func TestMana_onTheRecord(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "wren", nil)
	p.SpendMana(45)
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)

	back, err := FromRecord(p.Record(), &Recorder{}, cat, nil)
	require.NoError(t, err)
	assert.Equal(t, 55, back.CurrentMana())
}

func TestCooldown(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "wren", nil)
	assert.True(t, p.ReadyAt("heal").IsZero(), "never cast: ready")

	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	p.StartCooldown("heal", at)
	assert.Equal(t, at, p.ReadyAt("heal"))
	assert.True(t, p.ReadyAt("smite").IsZero(), "one ability's cooldown is its own")
}
