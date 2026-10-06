package sim

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/dice"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/testdice"
)

func content(t *testing.T) *loader.Content {
	t.Helper()
	c, err := loader.LoadContent(os.DirFS("../content"))
	require.NoError(t, err)
	return c
}

func TestKit(t *testing.T) {
	c := content(t)
	kit, err := StartingKit(c)
	require.NoError(t, err)
	require.NotEmpty(t, kit)

	p := kit.Player(c, 3)
	assert.Equal(t, 3, p.Power(), "everything made at the power asked")
	assert.NotNil(t, p.Equipment().At(rules.SlotWield), "the dagger is wielded")
	assert.Equal(t, p.MaxHealth(), p.CurrentHealth())

	for ref, why := range map[string]string{
		"wrathrock":           "want zone/id",
		"atlantis/sword":      "no zone",
		"wrathrock/nothing":   "no object",
		"wrathrock/waterskin": "nothing wears it",
	} {
		_, err := KitOf(c, []string{ref})
		assert.ErrorContains(t, err, why, ref)
	}
	_, err = KitOf(c, []string{"wrathrock/training_dagger", "wrathrock/short_sword"})
	assert.ErrorContains(t, err, "both go in")
}

// One fight, on loaded dice: the player swings first, every round.
func TestFight_playerSwingsFirst(t *testing.T) {
	c := content(t)
	kit, err := StartingKit(c)
	require.NoError(t, err)
	rabbit := mobile.NewInstance(c.Zones["wrathrock"].MobileDefinitions["rabbit"])

	d := testdice.New()
	d.Load([]int{20, 2}) // a natural 20, and 2 damage: the rabbit has 2
	o, err := Fight(d, kit.Player(c, 1), rabbit)
	require.NoError(t, err)
	assert.True(t, o.Won)
	assert.Equal(t, 1, o.Rounds)
	assert.Equal(t, 100, o.HealthLeft, "it never swung")
}

// Many fights: a sure thing is a sure thing, and a boss alone is hopeless.
func TestRun(t *testing.T) {
	c := content(t)
	kit, err := StartingKit(c)
	require.NoError(t, err)
	roller := dice.New([32]byte{7})

	easy, err := Run(roller, c, kit, 5, c.Zones["hollowfield"].MobileDefinitions["field_rat"], 50)
	require.NoError(t, err)
	assert.Equal(t, 50, easy.Fights)
	assert.Equal(t, 1.0, easy.WinRate())
	assert.Positive(t, easy.HealthLeft())

	boss, err := Run(roller, c, kit, 1, c.Zones["barrow"].MobileDefinitions["barrow_king"], 50)
	require.NoError(t, err)
	assert.Zero(t, boss.WinRate())
	assert.Positive(t, boss.Rounds())
}
