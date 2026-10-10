package world

import (
	"os"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/memstore"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/testdice"
)

// The real ladder: Hollowfields, then the Mill, then the Barrow, each with
// the walk there from Temple Square. A route that changes here is content
// that moved -- check the welcome line and the site guide say the same.
func TestGoals_theRealLadder(t *testing.T) {
	content, err := loader.LoadContent(os.DirFS("../content"))
	require.NoError(t, err)
	w, err := New(content, memstore.New(), testdice.New())
	require.NoError(t, err)
	rec := &player.Recorder{}
	p := player.NewTestPlayer(uuid.New(), "testdood", rec)
	w.PlacePlayer(p, w.StartRoom)

	goals := func(power int) event.Goals {
		t.Helper()
		p.Equipment().Unequip(rules.SlotHead)
		if power > 0 {
			d := object.NewDefinition("cap", "cap", "wrathrock", rules.ObjectCategoryArmor,
				nil, "a cap", "A cap.", rules.SlotHead, rules.ArmorTypeCloth, nil)
			inst := object.NewInstance(uuid.New(), d)
			inst.Power = power
			p.Equipment().Equip(rules.SlotHead, inst)
		}
		rec.Clear()
		require.NoError(t, w.HandleIncomingMessage(gameserver.NewHandlerParameter(gameserver.NewTestConn(p), command.Goals{})))
		return sent[event.Goals](t, rec, 0)
	}

	hollowfields := event.GoalZone{Name: "The Hollowfields", Min: 1, Max: 5, Route: "5s"}
	mill := event.GoalZone{Name: "The Drowned Mill", Min: 4, Max: 7, Route: "5s 2w"}
	barrow := event.GoalZone{Name: "The Sunken Barrow", Min: 6, Max: 10, Route: "9s e d"}

	g := goals(0)
	assert.Empty(t, g.Fits)
	assert.Equal(t, []event.GoalZone{hollowfields}, g.Next, "no gear: the bottom rung is next")
	assert.Equal(t, "Temple Square", g.From)

	g = goals(1)
	assert.Equal(t, []event.GoalZone{hollowfields}, g.Fits)
	assert.Equal(t, []event.GoalZone{mill}, g.Next)

	g = goals(5)
	assert.Equal(t, []event.GoalZone{hollowfields, mill}, g.Fits, "bands overlap")
	assert.Equal(t, []event.GoalZone{barrow}, g.Next)

	g = goals(11)
	assert.Empty(t, g.Fits)
	assert.Empty(t, g.Next)
}

func TestSpeedwalk(t *testing.T) {
	assert.Equal(t, "", speedwalk(nil))
	assert.Equal(t, "3n e 2d", speedwalk([]rules.Direction{rules.DirectionNorth, rules.DirectionNorth, rules.DirectionNorth,
		rules.DirectionEast, rules.DirectionDown, rules.DirectionDown}))
}
