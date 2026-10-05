-- The Barrow-King. Placeholder lines: rewrite them in his voice.
local openers = {
  "Who wakes the king beneath the oak?",
  "Kneel, %s. The barrow keeps what it takes.",
}
local taunts = {
  "Hah! You cannot defeat me!",
  "I have outlasted better than you.",
  "Your bones will guard my door.",
}

function on_fight_start(me, foe)
  -- a fresh fight gets a fresh guard: he regenerates to full between them
  me.memory.risen = nil
  me:say(string.format(pick(openers), foe.name))
end

function on_fight_pulse(me, foe)
  -- at half health, once a fight, his guard rises. No taunt that round:
  -- the line and the rising are the moment.
  if not me.memory.risen and me.health <= me.max_health / 2 then
    me.memory.risen = true
    me:say("Rise, my guard! Rise and defend your king!")
    me:summon("barrow_skeleton", 2)
    return
  end
  if chance(15) then
    local line = pick(taunts)
    if line ~= me.memory.last then
      me:say(line)
      me.memory.last = line
    end
  end
end
