-- A crow. It wanders the Hollowfields and can't resist anything left lying
-- in them: whatever it takes is in its corpse, for whoever catches it. The
-- engine decides what's fair game (world/scavenge.go): dropped and left a
-- minute, never a corpse, never a bag.
function on_arrive(me)
  me:take()
end
