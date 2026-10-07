-- The Drowned Miller. Placeholder lines: rewrite them in his voice.
local openers = {
  "Who comes down to the wheel?",
  "The river took me, %s. It'll take you too.",
}
local taunts = {
  "The wheel turns again!",
  "Down into the water with you.",
  "I ground better than you to flour.",
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
