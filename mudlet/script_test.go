package mudlet

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	lua "github.com/yuin/gopher-lua"
)

// fakeMudlet is as much of Mudlet's Lua API as watchmud.lua calls, recording
// what it's asked to do. Gauges and the mapper are tables whose methods note
// their arguments; the map is rooms with an area, coordinates and exits.
const fakeMudlet = `
calls = { gauges = {}, borders = {}, handlers = {}, centered = nil, updated = 0 }
gmcp = {}

local function styled()
  return { setStyleSheet = function(self, css) self.css = css end }
end

Geyser = {
  Gauge = { new = function(self, opts)
    local g = { name = opts.name, front = styled(), back = styled() }
    function g:setValue(cur, max, text) self.cur, self.max, self.text = cur, max, text end
    calls.gauges[opts.name] = g
    return g
  end },
  Mapper = { new = function(self, opts) calls.mapper = opts; return {} end },
}

function setBorderBottom(px) calls.borders.bottom = px end
function setBorderRight(px) calls.borders.right = px end

local nextHandler = 0
function registerAnonymousEventHandler(event, fn)
  nextHandler = nextHandler + 1
  calls.handlers[event] = fn
  return nextHandler
end
function killAnonymousEventHandler(id) end

areas, rooms = {}, {}
local nextArea = 0
function getAreaTable() return areas end
function addAreaName(name) nextArea = nextArea + 1; areas[name] = nextArea; return nextArea end
function roomExists(id) return rooms[id] ~= nil end
function addRoom(id) rooms[id] = { exits = {} }; return true end
function setRoomArea(id, area) rooms[id].area = area end
function getRoomArea(id) return rooms[id].area end
function setRoomName(id, name) rooms[id].name = name end
function setRoomCoordinates(id, x, y, z) rooms[id].x, rooms[id].y, rooms[id].z = x, y, z end
function getRoomCoordinates(id) local r = rooms[id]; return r.x, r.y, r.z end
function setExit(from, to, dir) rooms[from].exits[dir] = to end
function centerview(id) calls.centered = id end
function updateMap() calls.updated = calls.updated + 1 end

-- what Mudlet does when GMCP arrives: the table, then the event
function arrive(pkg, data)
  if pkg == "Char.Vitals" then gmcp.Char = { Vitals = data } end
  if pkg == "Room.Info" then gmcp.Room = { Info = data } end
  local fn = calls.handlers["gmcp." .. pkg]
  local f = _G
  for part in string.gmatch(fn, "[^.]+") do f = f[part] end
  f()
end
`

func loaded(t *testing.T) *lua.LState {
	t.Helper()
	L := lua.NewState()
	t.Cleanup(L.Close)
	require.NoError(t, L.DoString(fakeMudlet))
	require.NoError(t, L.DoString(Script), "watchmud.lua")
	return L
}

func eval(t *testing.T, L *lua.LState, expr string) lua.LValue {
	t.Helper()
	require.NoError(t, L.DoString("__result = "+expr))
	return L.GetGlobal("__result")
}

func TestScript_registersForGMCP(t *testing.T) {
	L := loaded(t)
	assert.Equal(t, "WatchMUD.onVitals", eval(t, L, `calls.handlers["gmcp.Char.Vitals"]`).String())
	assert.Equal(t, "WatchMUD.onRoom", eval(t, L, `calls.handlers["gmcp.Room.Info"]`).String())
	assert.Equal(t, lua.LTrue, eval(t, L, `calls.borders.bottom > 0 and calls.borders.right > 0`), "room made for the bars and the map")
}

// Loading it again -- a reinstall -- doesn't make a second set of bars.
func TestScript_loadsTwice(t *testing.T) {
	L := loaded(t)
	require.NoError(t, L.DoString(`first = WatchMUD.hp`))
	require.NoError(t, L.DoString(Script))
	assert.Equal(t, lua.LTrue, eval(t, L, `first == WatchMUD.hp`))
}

func TestScript_vitalsFillTheBars(t *testing.T) {
	L := loaded(t)
	require.NoError(t, L.DoString(`arrive("Char.Vitals", { hp = 97, maxhp = 100, mp = 80, maxmp = 100 })`))

	assert.Equal(t, "97", eval(t, L, `calls.gauges.WatchMUDHealth.cur`).String())
	assert.Equal(t, "100", eval(t, L, `calls.gauges.WatchMUDHealth.max`).String())
	assert.Contains(t, eval(t, L, `calls.gauges.WatchMUDHealth.text`).String(), "97 / 100 health")
	assert.Contains(t, eval(t, L, `calls.gauges.WatchMUDMana.text`).String(), "80 / 100 mana")
}

// Walking from the square to the market: two rooms, a step apart, joined
// both ways, and the map follows the player.
func TestScript_mapDrawsAsYouWalk(t *testing.T) {
	L := loaded(t)
	require.NoError(t, L.DoString(`
		arrive("Room.Info", { num = 100, name = "Temple Square", area = "Wrathrock", exits = { s = 200, e = 300 } })
		arrive("Room.Info", { num = 200, name = "Market Square", area = "Wrathrock", exits = { n = 100 } })
	`))

	assert.Equal(t, "Temple Square", eval(t, L, `rooms[100].name`).String())
	assert.Equal(t, "0 0 0", eval(t, L, `rooms[100].x .. " " .. rooms[100].y .. " " .. rooms[100].z`).String())
	assert.Equal(t, "0 -1 0", eval(t, L, `rooms[200].x .. " " .. rooms[200].y .. " " .. rooms[200].z`).String(), "south is down the screen")
	assert.Equal(t, "200", eval(t, L, `rooms[100].exits[6]`).String(), "square, south, to the market")
	assert.Equal(t, "100", eval(t, L, `rooms[200].exits[1]`).String(), "market, north, to the square")
	assert.Equal(t, lua.LNil, eval(t, L, `rooms[100].exits[4]`), "no exit drawn to a room never seen")
	assert.Equal(t, "200", eval(t, L, `calls.centered`).String())
	assert.Equal(t, "Wrathrock", eval(t, L, `(function() for n, id in pairs(areas) do if id == rooms[200].area then return n end end end)()`).String())
}

// A room seen before its neighbour gets linked when the neighbour turns up,
// and a new area starts its own map.
func TestScript_linksLateAndNewAreas(t *testing.T) {
	L := loaded(t)
	require.NoError(t, L.DoString(`
		arrive("Room.Info", { num = 1, name = "southern path", area = "Wrathrock", exits = { s = 2 } })
		arrive("Room.Info", { num = 2, name = "The Waystone", area = "The Hollowfields", exits = { n = 1, w = 3 } })
		arrive("Room.Info", { num = 3, name = "The Millpond", area = "The Hollowfields", exits = { e = 2 } })
	`))

	assert.Equal(t, "2", eval(t, L, `rooms[1].exits[6]`).String(), "linked across the zone line")
	assert.Equal(t, lua.LFalse, eval(t, L, `rooms[1].area == rooms[2].area`))
	assert.Equal(t, "0 0", eval(t, L, `rooms[2].x .. " " .. rooms[2].y`).String(), "the first room of a new area is its origin")
	assert.Equal(t, "-1 0", eval(t, L, `rooms[3].x .. " " .. rooms[3].y`).String(), "west of it")
}

// The package is what Mudlet installs: config.lua naming it, and an XML file
// of that name holding the script, intact.
func TestBuild(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, Build(&buf))
	z, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)

	files := map[string]string{}
	for _, f := range z.File {
		r, err := f.Open()
		require.NoError(t, err)
		b, err := io.ReadAll(r)
		require.NoError(t, err)
		files[f.Name] = string(b)
	}
	require.Contains(t, files, "config.lua")
	assert.Contains(t, files["config.lua"], `mpackage = "WatchMUD"`)
	assert.Contains(t, files["config.lua"], `version = "1"`)

	var pkg mudletPackage
	require.NoError(t, xml.Unmarshal([]byte(files[Name+".xml"]), &pkg))
	require.Len(t, pkg.ScriptPackage.Scripts, 1)
	assert.Equal(t, Script, pkg.ScriptPackage.Scripts[0].Script, "the script survives the escaping")

	// and config.lua is Lua
	L := lua.NewState()
	defer L.Close()
	require.NoError(t, L.DoString(files["config.lua"]))
	assert.Equal(t, "WatchMUD", L.GetGlobal("mpackage").String())
}
