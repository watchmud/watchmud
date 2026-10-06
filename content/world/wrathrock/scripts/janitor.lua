-- The town janitor. He can't be fought (nofight); he wanders Wrathrock, and
-- whatever players dropped and left lying he sweeps up for good -- the engine
-- decides what's junk (world/janitor.go): never a corpse, never the donation
-- room. Placeholder lines: rewrite them in his voice.
local grumbles = {
  "Every day. Every single day.",
  "Do I look like I've got nothing better to do?",
  "Somebody's mother didn't raise them right.",
}

function on_arrive(me)
  if me:junk() > 0 then
    wait(2)
    if me:sweep(3) > 0 and chance(30) then
      me:say(pick(grumbles))
    end
  end
end
