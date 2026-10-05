-- The test world's summoner: two imps the moment a fight starts.
function on_fight_start(me, foe)
  me:summon("imp", 2)
end
