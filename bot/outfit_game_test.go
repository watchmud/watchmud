package bot

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
)

// The kit against the real content. Each piece is something the General
// Store sells under that name, and the first thing in its stock its keyword
// buys -- buy takes the first match. The keyword names nothing else the bot
// could be carrying or wearing, so wear puts on what it bought. And every alias
// it removes by is on a piece of the starting kit, in its short description (so
// the equipment list shows which is on), and on no piece of the kit (so
// removing it never takes off a bought one).
func TestKit_theStoreSellsIt(t *testing.T) {
	content, err := loader.LoadContent(os.DirFS("../content"))
	require.NoError(t, err)
	shop := content.Zones["wrathrock"].Shops["general_store"]
	require.NotNil(t, shop)
	var starting []*object.Definition
	for _, g := range content.Catalog.StartingGear {
		starting = append(starting, content.Zones[g.ZoneId].ObjectDefinitions[g.DefinitionId])
	}
	kitDefs := map[string]*object.Definition{}
	for _, p := range kit {
		for _, s := range shop.Stock {
			if s.Object.Name == p.name {
				kitDefs[p.name] = s.Object
			}
		}
		require.NotNil(t, kitDefs[p.name], "the store doesn't sell %q", p.name)
	}

	for _, p := range kit {
		assert.NotContains(t, p.keyword, " ", "buy and wear take one word")
		for _, s := range shop.Stock {
			if s.Object.Matches(p.keyword) {
				assert.Equal(t, p.name, s.Object.Name, "buy %s buys the first match", p.keyword)
				break
			}
		}
		for _, q := range kit {
			if q.name != p.name {
				assert.False(t, kitDefs[q.name].Matches(p.keyword), "%q is %s's too", p.keyword, q.name)
			}
		}
		for _, d := range starting {
			assert.False(t, d.Matches(p.keyword), "%q is the starting %s's too", p.keyword, d.Name)
		}
	}

	for _, p := range kit {
		if p.replaces == "" {
			continue
		}
		found := false
		for _, d := range starting {
			if d.HasAlias(p.replaces) && strings.Contains(d.ShortDescription, p.replaces) {
				found = true
			}
		}
		assert.True(t, found, "%q names nothing in the starting kit", p.replaces)
		for _, d := range kitDefs {
			assert.False(t, d.Matches(p.replaces), "%q also takes off %s", p.replaces, d.Name)
		}
	}
}

// The text the readers read, as the real server writes it.
func TestOutfit_readsTheRealText(t *testing.T) {
	addr, g := startGameStore(t, time.Hour)
	createCharacter(t, addr, "Quillon", "correcthorse")
	setRecord(t, g, "Quillon", func(r *player.Record) { r.Coins = 100 })
	c := loginAs(t, addr, "Quillon", "correcthorse")
	require.NoError(t, recall(c))

	require.NoError(t, c.Send("equipment"))
	m, err := c.Expect(`(?s)(Equipment \(power \d+\)\n.*?)`+prompt, 5*time.Second)
	require.NoError(t, err)
	rows := readEquipment(m[1])
	require.Len(t, rows, 4, m[1])
	assert.Equal(t, "arming sword", nextPiece(rows).name)

	require.NoError(t, c.Send("inventory"))
	m, err = c.Expect(`(?s)((?:Inventory|You aren't carrying anything\.)\n.*?)`+prompt, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, 100, readPurse(m[1]))

	for _, s := range []step{waitIn, {"west", "Smithy"}} {
		require.NoError(t, c.Send(s.dir))
		_, err := room(c, s.room)
		require.NoError(t, err)
	}
	require.NoError(t, c.Send("repair all"))
	m, err = c.Expect(`(?s)(.*?)`+prompt, 5*time.Second)
	require.NoError(t, err)
	assert.True(t, repairRe.MatchString(m[1]), m[1])

	for _, s := range []step{{"east", "Market Square"}, {"east", "General Store"}} {
		require.NoError(t, c.Send(s.dir))
		_, err := room(c, s.room)
		require.NoError(t, err)
	}
	require.NoError(t, c.Send("buy arming"))
	m, err = c.Expect(`(?s)(.*?)`+prompt, 5*time.Second)
	require.NoError(t, err)
	bought := buyRe.FindStringSubmatch(m[1])
	require.NotNil(t, bought, m[1])
	assert.NotEmpty(t, bought[1])
	require.NoError(t, c.Send("buy quilted"))
	m, err = c.Expect(`(?s)(.*?)`+prompt, 5*time.Second)
	require.NoError(t, err)
	refused := buyRe.FindStringSubmatch(m[1])
	require.NotNil(t, refused, m[1])
	assert.NotEmpty(t, refused[2], "100 coins buys one piece, not two")

	// each reply read through its prompt, which ends the line before it: a
	// reply after one starts mid-line, where the bot's own chunks never do
	require.NoError(t, c.Send("remove training"))
	m, err = c.Expect(`(?s)(.*?)`+prompt, 5*time.Second)
	require.NoError(t, err)
	assert.True(t, removedRe.MatchString(m[1]), m[1])
	require.NoError(t, c.Send("wear arming"))
	m, err = c.Expect(`(?s)(.*?)`+prompt, 5*time.Second)
	require.NoError(t, err)
	assert.True(t, woreRe.MatchString(m[1]), m[1])
}

// A hunter with 200 coins buys the sword and the jerkin -- 80 each -- on its
// way out, is refused the cap, and is power 2. And then it goes hunting.
func TestAdventurer_buysItsKit(t *testing.T) {
	addr, g := startGameStore(t, 10*time.Millisecond)
	createCharacter(t, addr, "Wren", "correcthorse")
	setRecord(t, g, "Wren", func(r *player.Record) { r.Coins = 200 })
	a := runAdventurer(t, addr, "Wren")

	ok := assert.Eventually(t, func() bool { return a.Stats().Kills >= 1 }, 60*time.Second, 50*time.Millisecond)
	st := a.Stats()
	require.True(t, ok, "stats: %+v", st)
	assert.Equal(t, 2, st.Bought)
	assert.Equal(t, 2, st.Power, "sword 4, jerkin 4, cloak 1, token 1")
}

// A dagger worn to a fifth goes to the smith before the bot goes hunting, and
// a purse too light for the sword learns its price on the way.
func TestAdventurer_repairsFirst(t *testing.T) {
	addr, g := startGameStore(t, 10*time.Millisecond)
	createCharacter(t, addr, "Wren", "correcthorse")
	setRecord(t, g, "Wren", func(r *player.Record) {
		r.Coins = 50
		for i := range r.Inventory {
			if r.Inventory[i].DefinitionId == "training_dagger" {
				d := 5
				r.Inventory[i].Durability = &d
			}
		}
	})
	a := runAdventurer(t, addr, "Wren")

	ok := assert.Eventually(t, func() bool { return a.Stats().Kills >= 1 }, 60*time.Second, 50*time.Millisecond)
	st := a.Stats()
	require.True(t, ok, "stats: %+v", st)
	assert.Equal(t, 1, st.Repairs)
	assert.Zero(t, st.Bought)
}
