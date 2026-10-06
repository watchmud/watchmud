# The Drowned Mill

Status: design, 2026-10-06. LEVELS.md section 4, "Content": a zone between the
Hollowfields (1-5) and the Sunken Barrow (6-10).

## Why

A player who has outgrown the farms has nowhere to go but the barrow, whose grave rats
start at 6 and whose skeletons are 8. The mill is the step between: power 4-7, gear at
those powers for all three kinds of player, and a small boss at 8 who is the barrow's
first lesson -- beatable by two people who know what they're doing.

## Where

West of the Millpond. The Waystone has said MILL to the west since the Hollowfields were
written; the mill whose wheel stopped is finally somewhere to go. Eight rooms:

- **outside**: the Mill Yard (from the Millpond), the Sluice Gate, the Mill Race, the
  Reed Marsh
- **inside**: the Mill House, the Grain Loft (up), the Flooded Cellar (down), and the
  Wheel Pit below the wheel, where the miller is

## Who

| Mob | Power | HP | AC | Damage | Aggressive | Where |
|---|---|---|---|---|---|---|
| marsh eel | 4 | 22 | 12 | 1d6 | no | sluice, race |
| reed stalker | 5 | 28 | 13 | 1d6 | yes | marsh |
| drowned millhand | 6 | 34 | 13 | 1d8 | yes | mill house, loft, cellar |
| Drowned Miller | 8 | 90 | 14 | 2d6 | yes | wheel pit |

The miller has a script: lines at the start of a fight and now and then in it. No
summons -- that's the King's.

## What drops

One ability per item, as LEVELS.md keeps them apart; one item for each kind of player.

| Item | Slot | From | Grants |
|---|---|---|---|
| eelskin boots | feet, leather | eel 20% | -- |
| stalker's spear | wield, 1d8 | stalker 15% | smite |
| millhand's gloves | hands, leather | millhand 20% | -- |
| river-stone pendant | neck | millhand 10%, miller 35% | heal |
| miller's mallet | wield, 1d10 | miller 35% | stun |
| iron gauntlets | hands, plate | miller 35% | provoke |

## Wanderers

Everything aggressive here is above what a power-1 wanderer survives, so the door from
the Millpond west goes in `keepOut` -- `TestKeepOut_safe` insists on it.

## Tuning

Every number above is a first guess. The ones to watch: the miller's 90 health and 2d6
(should take two players at power 6-7 a real fight), and the stalker being aggressive
in a room a power-5 player walks into.
