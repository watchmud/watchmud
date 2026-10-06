# Groups: follow, group, gtell

Status: design, 2026-10-06; agreed in chat ("1, then 2"), with the choices below made
by Claude and open to change.

## Goal

The Drowned Miller and the Barrow-King are fights for two or more, and today two
players have nothing to group *with* but walking the same way and typing `tell`. A
group is a leader and the people following them: they move together, talk on their
own channel, and can see how each other are doing.

## Following

- `follow <player>` (in the same room) makes you follow them. Nobody's consent is
  needed -- as in every Diku -- but the one followed is told, and can be rid of you.
- **One level.** Following someone who is themselves following makes you follow
  *their* leader, so a group is always a leader and a flat list of followers, in the
  order they joined. No chains, so no cycles to detect and no "who leads whom" puzzles.
  A leader who starts following someone brings nobody along: their followers are told
  and stop.
- `follow` alone, or `follow <your own name>`, stops following.
- `ungroup <player>` (the leader) stops someone following them; `ungroup` alone
  disbands the whole group.
- **Walking pulls followers.** When the leader walks through an exit, each follower who
  was in the room they left, isn't fighting, and finds the way still passable walks
  after them, in join order, told "You follow Ann north." and shown the room. The
  rooms see each of them leave and arrive as they do now. A follower who can't come
  (fighting) stays behind, still following, and is told.
- **Only walking.** Recall, flee, dying and wizard teleports move one player; nobody
  is dragged across the world by a recall.
- **In memory.** A group is per session: logging out leaves it (a leader's followers
  are told the group is over). Nothing is saved.

## Group commands

- `group` lists the group: the leader first, then followers, each with health and mana
  and the room they're in -- what a healer needs to know who to heal.
- `gtell <message>` (`gt`) tells the whole group, wherever they are. Not in a group:
  "You aren't in a group."

## Not now

Shared loot or coins; a group's own XP (there is none); auto-assist (a follower joining
the leader's fight); following mobs or mobs following players; consent before being
followed. Each is a later call once groups are used.

## Code

- `world/group.go`: `World.leaderOf map[*player.Player]*player.Player` and
  `World.followers map[*player.Player][]*player.Player`, written only by
  `follow`/`unfollow`, cleared for a player by `RemovePlayer`.
- `handleMove` calls `w.followersCome(leader, from, dir)` after the leader's move.
- Commands `Follow{Target}`, `Ungroup{Target}`, `Group{}`, `GroupTell{Value}`; events
  `Following{Follower, Leader, Stopped}`, `FollowedOut{Leader, Direction}`,
  `GroupList{...}`, `GroupTold{Speaker, Value}`; codes `NOT_IN_GROUP`,
  `NOT_FOLLOWING_YOU`.
- Bots ignore all of it.
