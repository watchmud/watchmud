-- The hedge-witch. She pays you no mind -- until you ask about the one thing
-- she knows. Placeholder lines: rewrite them in her voice.
local asked = { heal = true, healing = true, hurt = true, wounded = true, mend = true }

function on_hear(me, speaker, said)
  for word in pairs(asked) do
    if said.words[word] then
      wait(1)
      me:say("Heal, is it? Sit out of the fight a while and it comes back on its own.")
      me:say("Or hold a censer and cast heal. I keep one. You'd have to take it off me, " .. speaker.name .. ".")
      return
    end
  end
end
