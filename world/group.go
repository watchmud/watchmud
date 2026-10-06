package world

import (
	"slices"

	"github.com/watchmud/watchmud/combat"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
)

// groups is who follows whom: a leader and a flat list of followers, in the
// order they joined. One level only -- following a follower follows their
// leader -- so there are no chains and no cycles. In memory, per session.
// Only follow, unfollow and disband write it.
type groups struct {
	leaderOf  map[*player.Player]*player.Player
	followers map[*player.Player][]*player.Player
	// noAssist is who has turned assist off; everyone else joins their
	// group's fights.
	noAssist map[*player.Player]bool
}

func newGroups() *groups {
	return &groups{
		leaderOf:  map[*player.Player]*player.Player{},
		followers: map[*player.Player][]*player.Player{},
		noAssist:  map[*player.Player]bool{},
	}
}

func (g *groups) follow(f, leader *player.Player) {
	g.leaderOf[f] = leader
	g.followers[leader] = append(g.followers[leader], f)
}

// unfollow stops f following anyone, and answers who they were following.
func (g *groups) unfollow(f *player.Player) *player.Player {
	leader := g.leaderOf[f]
	if leader == nil {
		return nil
	}
	delete(g.leaderOf, f)
	g.followers[leader] = slices.DeleteFunc(g.followers[leader], func(p *player.Player) bool { return p == f })
	if len(g.followers[leader]) == 0 {
		delete(g.followers, leader)
	}
	return leader
}

// disband stops everyone following leader, and answers who was.
func (g *groups) disband(leader *player.Player) []*player.Player {
	was := g.followers[leader]
	for _, f := range was {
		delete(g.leaderOf, f)
	}
	delete(g.followers, leader)
	return was
}

// members is p's group, leader first, or nil if p isn't in one.
func (g *groups) members(p *player.Player) []*player.Player {
	leader := p
	if l := g.leaderOf[p]; l != nil {
		leader = l
	}
	if len(g.followers[leader]) == 0 {
		return nil
	}
	return append([]*player.Player{leader}, g.followers[leader]...)
}

// stopFollowing ends p's follow, telling both of them.
func (w *World) stopFollowing(p *player.Player) {
	if leader := w.groups.unfollow(p); leader != nil {
		e := event.Following{Follower: p.Name(), Leader: leader.Name(), Stopped: true}
		p.Send(e)
		leader.Send(e)
	}
}

// disband ends every follow of leader, telling each of them and the leader.
func (w *World) disband(leader *player.Player) {
	for _, f := range w.groups.disband(leader) {
		e := event.Following{Follower: f.Name(), Leader: leader.Name(), Stopped: true}
		f.Send(e)
		leader.Send(e)
	}
}

// leaveGroups takes a player leaving the world out of any group, from either end.
func (w *World) leaveGroups(p *player.Player) {
	w.stopFollowing(p)
	w.disband(p)
	delete(w.groups.noAssist, p)
}

// assist brings the rest of member's group into a fight with foe: whoever is
// in the same room, not already fighting, and hasn't turned assist off. Only
// against a mob -- nothing else is fought -- and joining sets off no assist
// of its own, so there's no chain to follow.
func (w *World) assist(member, foe combat.Combatant) {
	p, ok := member.(*player.Player)
	if !ok {
		return
	}
	mob, ok := foe.(*mobile.Instance)
	if !ok {
		return
	}
	room := w.playerRoom(p)
	for _, other := range w.groups.members(p) {
		if other == p || w.playerRoom(other) != room || w.fightLedger.InFight(other) || w.groups.noAssist[other] {
			continue
		}
		if err := w.joinFight(other, mob); err != nil {
			other.Log().Warn().Err(err).Msg("assist")
			continue
		}
		room.Send(event.Assisted{Actor: other.Name(), Member: p.Name(), Target: mob.Name()})
	}
}

func (w *World) handleAssist(msg *gameserver.HandlerParameter, cmd command.Assist) {
	on := w.groups.noAssist[msg.Player]
	switch cmd.Setting {
	case "":
	case "on":
		on = true
	case "off":
		on = false
	default:
		msg.Fail(event.BadRequest)
		return
	}
	if on {
		delete(w.groups.noAssist, msg.Player)
	} else {
		w.groups.noAssist[msg.Player] = true
	}
	msg.Player.Send(event.AssistSet{On: on})
}

func (w *World) handleFollow(msg *gameserver.HandlerParameter, cmd command.Follow) {
	me := msg.Player
	if cmd.Target == "" || player.NameKey(cmd.Target) == player.NameKey(me.Name()) {
		if w.groups.leaderOf[me] == nil {
			msg.Fail(event.NotFollowing)
			return
		}
		w.stopFollowing(me)
		return
	}
	target, found := w.playerRoom(me).FindPlayer(cmd.Target)
	if !found {
		msg.Fail(event.ToPlayerNotFound)
		return
	}
	leader := target
	if l := w.groups.leaderOf[target]; l != nil {
		leader = l
	}
	switch {
	case leader == me:
		msg.Fail(event.FollowsYou)
		return
	case w.groups.leaderOf[me] == leader:
		msg.Fail(event.AlreadyFollowing)
		return
	}
	// a leader who goes off to follow someone else takes nobody along
	w.disband(me)
	w.stopFollowing(me)
	w.groups.follow(me, leader)
	e := event.Following{Follower: me.Name(), Leader: leader.Name()}
	me.Send(e)
	leader.Send(e)
}

func (w *World) handleUngroup(msg *gameserver.HandlerParameter, cmd command.Ungroup) {
	me := msg.Player
	if len(w.groups.followers[me]) == 0 {
		msg.Fail(event.NoFollowers)
		return
	}
	if cmd.Target == "" {
		w.disband(me)
		return
	}
	key := player.NameKey(cmd.Target)
	for _, f := range w.groups.followers[me] {
		if player.NameKey(f.Name()) == key {
			w.stopFollowing(f)
			return
		}
	}
	msg.Fail(event.NotFollowingYou)
}

func (w *World) handleGroup(msg *gameserver.HandlerParameter, cmd command.Group) {
	members := w.groups.members(msg.Player)
	if members == nil {
		msg.Fail(event.NotInGroup)
		return
	}
	list := event.GroupList{}
	for i, p := range members {
		m := event.GroupMember{
			Name:      p.Name(),
			Leader:    i == 0,
			Health:    p.CurrentHealth(),
			MaxHealth: p.MaxHealth(),
			Mana:      p.CurrentMana(),
			MaxMana:   p.MaxMana(),
		}
		if r := w.playerRoom(p); r != nil {
			m.Room = r.Name
		}
		list.Members = append(list.Members, m)
	}
	msg.Player.Send(list)
}

func (w *World) handleGroupTell(msg *gameserver.HandlerParameter, cmd command.GroupTell) {
	members := w.groups.members(msg.Player)
	if members == nil {
		msg.Fail(event.NotInGroup)
		return
	}
	if cmd.Value == "" {
		msg.Fail(event.NoValue)
		return
	}
	for _, p := range members {
		p.Send(event.GroupTold{Speaker: msg.Player.Name(), Value: cmd.Value})
	}
}

// followersCome walks leader's followers after them, from the room the leader
// just left: each one there and not fighting, in the order they joined. A
// follower in a fight stays behind, still following, and is told.
func (w *World) followersCome(leader *player.Player, from *spaces.Room, dir rules.Direction, dest *spaces.Room) {
	for _, f := range slices.Clone(w.groups.followers[leader]) {
		if w.playerRoom(f) != from {
			continue
		}
		if w.fightLedger.InFight(f) {
			f.Send(event.Followed{Leader: leader.Name(), Direction: dir, Fighting: true})
			continue
		}
		w.movePlayer(f, dir, dest)
		f.Send(event.Followed{Leader: leader.Name(), Direction: dir})
		f.Send(dest.DescriptionExcept(f))
	}
}
