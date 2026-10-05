-- The test world's waiter: a beat after a fight starts, one imp.
function on_fight_start(me, foe)
  wait(1)
  me:summon("imp", 1)
end
