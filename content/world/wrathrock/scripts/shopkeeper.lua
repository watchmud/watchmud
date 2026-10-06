-- The General Store's shopkeeper. Can't be fought (nofight); answers what's
-- asked about her trade, and greets a hello. Placeholder lines: rewrite them
-- in her voice. Every command she names must be one the game knows.
local topics = {
  { words = { "buy", "list", "sale", "stock", "wares", "price", "prices", "cost" },
    line = "Everything I sell is on the 'list'. Point at what you like: 'buy satchel'." },
  { words = { "sell", "selling", "worth", "value", "pay" },
    line = "I'll buy most things, at a fair price for used. Ask first with 'value', then 'sell'." },
  { words = { "repair", "repairs", "broken", "mend", "fix" },
    line = "Mending's the smith's trade, not mine. West of Market Square." },
  { words = { "bag", "bags", "satchel" },
    line = "The satchel holds ten things, and none of them are coins. Mind you don't sell it full." },
}
local hellos = { hello = true, hi = true, greetings = true, morning = true, evening = true }

function on_hear(me, speaker, said)
  for _, topic in ipairs(topics) do
    for _, word in ipairs(topic.words) do
      if said.words[word] then
        me:say(topic.line)
        return
      end
    end
  end
  for word in pairs(hellos) do
    if said.words[word] then
      me:say("Welcome in, " .. speaker.name .. ". Have a look at the 'list'.")
      return
    end
  end
end
