-- WatchMUD for Mudlet: health and mana bars, and a map that draws itself.
--
-- Both are built from GMCP, which the server sends any client that asks
-- (telnet/gmcp.go): Char.Vitals for the bars, Room.Info for the map. Nothing
-- here reads the game's text, so a change of wording can't break it.
--
-- Safe to load twice: everything lives in the WatchMUD table, and a second
-- load replaces the handlers rather than adding another set.

WatchMUD = WatchMUD or {}
local W = WatchMUD

W.version = "2"

-- ---- the bars --------------------------------------------------------------

local barHeight = 24

-- Geyser draws in the main window's border, which this makes room for.
setBorderBottom(barHeight * 2 + 12)

local function gauge(name, y, color)
  local g = Geyser.Gauge:new({
    name = name,
    x = "0%", y = y,
    width = "50%", height = barHeight .. "px",
  })
  g.front:setStyleSheet("background-color: " .. color .. "; border-radius: 4px; margin: 1px;")
  g.back:setStyleSheet("background-color: #222222; border-radius: 4px; margin: 1px;")
  return g
end

W.hp = W.hp or gauge("WatchMUDHealth", -(barHeight * 2 + 8) .. "px", "#2e8b57")
W.mp = W.mp or gauge("WatchMUDMana", -(barHeight + 4) .. "px", "#3a6ea5")

function W.onVitals()
  local v = gmcp.Char.Vitals
  local hp, maxhp = tonumber(v.hp), tonumber(v.maxhp)
  local mp, maxmp = tonumber(v.mp), tonumber(v.maxmp)
  W.hp:setValue(hp, math.max(maxhp, 1), string.format("<center>%d / %d health</center>", hp, maxhp))
  W.mp:setValue(mp, math.max(maxmp, 1), string.format("<center>%d / %d mana</center>", mp, maxmp))
end

-- ---- the map ---------------------------------------------------------------

-- Mudlet numbers its exits; these are the ones Room.Info names (telnet/gmcp.go
-- sends the abbreviation: "n", "u", ...).
local exitNumber = { n = 1, ne = 2, nw = 3, e = 4, w = 5, s = 6, se = 7, sw = 8, u = 9, d = 10 }

-- Where a step in each direction goes on the map. North is up the screen.
local step = {
  n = { 0, 1, 0 }, s = { 0, -1, 0 }, e = { 1, 0, 0 }, w = { -1, 0, 0 },
  u = { 0, 0, 1 }, d = { 0, 0, -1 },
}

-- The map sits on the right of the main window, in a border made for it.
local mapWidth = 360
setBorderRight(mapWidth + 10)
W.map = W.map or Geyser.Mapper:new({
  name = "WatchMUDMap",
  x = -mapWidth .. "px", y = "0px",
  width = mapWidth .. "px", height = "60%",
})

-- Every room's exits as the server last told them, room number -> {dir -> to}.
-- Mudlet keeps its own map between sessions; this only has to cover rooms seen
-- in this one, to link a room to neighbours seen before it was.
W.exits = W.exits or {}

local function areaId(name)
  local id = getAreaTable()[name]
  if not id then
    id = addAreaName(name)
  end
  return id
end

-- place puts a room where the server says it is on its area's grid (Room.Info
-- "grid"). A server too old to say gets the guess: next to a neighbour it's
-- already got, in the same area, or at the area's origin when it has none --
-- which a room arrived at by recall or login hasn't, so it can land on another.
local function place(num, info, area)
  if info.grid then
    setRoomCoordinates(num, tonumber(info.grid.x), tonumber(info.grid.y), tonumber(info.grid.z))
    return
  end
  for dir, to in pairs(info.exits or {}) do
    if roomExists(to) and getRoomArea(to) == area and step[dir] then
      local x, y, z = getRoomCoordinates(to)
      local d = step[dir]
      setRoomCoordinates(num, x - d[1], y - d[2], z - d[3])
      return
    end
  end
  for from, exits in pairs(W.exits) do
    for dir, to in pairs(exits) do
      if to == num and roomExists(from) and getRoomArea(from) == area and step[dir] then
        local x, y, z = getRoomCoordinates(from)
        local d = step[dir]
        setRoomCoordinates(num, x + d[1], y + d[2], z + d[3])
        return
      end
    end
  end
  setRoomCoordinates(num, 0, 0, 0)
end

local function link(from, exits)
  for dir, to in pairs(exits) do
    if exitNumber[dir] and roomExists(to) then
      setExit(from, to, exitNumber[dir])
    end
  end
end

function W.onRoom()
  local info = gmcp.Room.Info
  local num = tonumber(info.num)
  local exits = {}
  for dir, to in pairs(info.exits or {}) do
    exits[dir] = tonumber(to)
  end
  info = { name = info.name, area = info.area, exits = exits, grid = info.grid }

  if not roomExists(num) then
    addRoom(num)
    local area = areaId(info.area)
    setRoomArea(num, area)
    place(num, info, area)
  elseif info.grid then
    -- every visit: a map drawn by guessing, before the server said, mends
    place(num, info, getRoomArea(num))
  end
  setRoomName(num, info.name)
  W.exits[num] = exits

  -- this room's ways out, and every room seen before whose way leads here
  link(num, exits)
  for from, ex in pairs(W.exits) do
    for dir, to in pairs(ex) do
      if to == num and exitNumber[dir] then
        setExit(from, num, exitNumber[dir])
      end
    end
  end

  centerview(num)
  updateMap()
end

-- ---- wiring ----------------------------------------------------------------

if W.handlers then
  for _, id in ipairs(W.handlers) do
    killAnonymousEventHandler(id)
  end
end
-- Uninstalled, it takes itself away: the handlers, the bars, the map and the
-- room it made for them. (Mudlet keeps the map it drew, as it keeps any.)
function W.onUninstall(_, package)
  if package ~= "WatchMUD" then
    return
  end
  for _, id in ipairs(W.handlers or {}) do
    killAnonymousEventHandler(id)
  end
  W.hp:hide()
  W.mp:hide()
  W.map:hide()
  setBorderBottom(0)
  setBorderRight(0)
  WatchMUD = nil
end

W.handlers = {
  registerAnonymousEventHandler("gmcp.Char.Vitals", "WatchMUD.onVitals"),
  registerAnonymousEventHandler("gmcp.Room.Info", "WatchMUD.onRoom"),
  registerAnonymousEventHandler("sysUninstall", "WatchMUD.onUninstall"),
}
