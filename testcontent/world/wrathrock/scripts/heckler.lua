-- The test world's one scripted mob. Its opener proves memory: the first
-- fight gets "Come on then", every later one with the same heckler "You again".
local taunts = { "Is that all?", "My grandmother hits harder." }

function on_fight_start(me, foe)
  if me.memory.met then
    me:say("You again, " .. foe.name .. "?")
  else
    me.memory.met = true
    me:say("Come on then, " .. foe.name .. "!")
  end
end

function on_fight_pulse(me, foe)
  if chance(50) then
    me:say(pick(taunts))
  end
end

function on_hear(me, speaker, said)
  if said.words.hello then
    me:say("Hello yourself, " .. speaker.name .. ".")
  end
end
