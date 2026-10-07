-- A bandit. Brave enough until it's losing: once, below a quarter of its
-- health, it may beg off and run. Placeholder lines and odds -- whether a kill
-- that gets away (with its loot) is fun is a tuning call.
local pleas = {
  "Not worth dying for!",
  "Keep it, keep it!",
}

function on_fight_pulse(me, foe)
  if me.memory.ran or me.health * 4 > me.max_health then
    return
  end
  me.memory.ran = true
  if chance(50) then
    me:say(pick(pleas))
    me:flee()
  end
end
