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
  me:say(string.format(pick(openers), foe.name))
end

function on_fight_pulse(me, foe)
  if chance(15) then
    local line = pick(taunts)
    if line ~= me.memory.last then
      me:say(line)
      me.memory.last = line
    end
  end
end
