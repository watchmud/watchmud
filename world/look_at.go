package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

// lookAt is "look <thing>": a player or a mob in the room by name, then
// something carried or worn, then something on the floor. Only the one
// looking is told.
func (w *World) lookAt(msg *gameserver.HandlerParameter, name string) {
	room := w.playerRoom(msg.Player)
	if p, found := room.FindPlayer(name); found {
		msg.Player.Send(w.lookedAtPlayer(p))
		return
	}
	if mob, found := room.FindMobile(name); found {
		msg.Player.Send(w.lookedAtMob(mob))
		return
	}
	target, err := parseTarget(name)
	if err != nil {
		log.Debug().Err(err).Str("player", msg.Player.Name()).Str("target", name).Msg("look: can't parse target")
		msg.Fail(event.ParseError)
		return
	}
	found := targetsIn(target, msg.Player.Inventory().All())
	if len(found) == 0 {
		found = targetsIn(target, room.Inventory.All())
	}
	if len(found) == 0 {
		msg.Fail(event.TargetNotFound)
		return
	}
	msg.Player.Send(w.lookedAtObject(found[0], msg.Player.Equipment().ItemEquipped(found[0])))
}

func (w *World) lookedAtObject(inst *object.Instance, worn bool) event.LookedAtObject {
	d := inst.Definition
	l := event.LookedAtObject{
		Item:          d.ShortDescription,
		Slot:          d.EquipmentSlot,
		ArmorType:     d.ArmorType,
		Worn:          worn,
		Power:         inst.Power,
		Durability:    inst.Durability,
		MaxDurability: d.MaxDurability,
		Broken:        inst.Broken(),
		Container:     inst.Contents != nil,
		Closed:        inst.Closed(),
		Locked:        inst.Lock != nil && inst.Lock.Locked,
	}
	if d.EquipmentSlot == rules.SlotWield {
		l.Damage = string(d.Damage)
	}
	if d.ArmorType != rules.ArmorTypeNone {
		l.Armor = w.content.Catalog.ArmorWeight(d.ArmorType, d.EquipmentSlot)
	}
	for _, id := range d.Abilities {
		if a := w.content.Catalog.Abilities[id]; a != nil {
			l.Abilities = append(l.Abilities, a.Name)
		}
	}
	if inst.Contents != nil {
		l.Holding = inst.Contents.Len()
		if d.Container != nil {
			l.Capacity = d.Container.Capacity
		}
	}
	return l
}

func (w *World) lookedAtMob(mob *mobile.Instance) event.LookedAtMob {
	l := event.LookedAtMob{Name: mob.Name(), Health: percent(mob.CurHealth, mob.Definition.MaxHealth)}
	if fight := w.fightLedger.GetFight(mob); fight != nil {
		l.Fighting = fight.Fightee.Name()
	}
	return l
}

func (w *World) lookedAtPlayer(p *player.Player) event.LookedAtPlayer {
	l := event.LookedAtPlayer{
		Name:    p.Name(),
		Lineage: p.LineageName(),
		Role:    w.roleName(p.RoleWeights()),
		Health:  percent(p.CurrentHealth(), p.MaxHealth()),
	}
	if fight := w.fightLedger.GetFight(p); fight != nil {
		l.Fighting = fight.Fightee.Name()
	}
	for _, inst := range p.Equipment().All() {
		l.Wearing = append(l.Wearing, inst.Definition.ShortDescription)
	}
	return l
}

// percent is cur of max as a whole percentage, 0 for nothing out of nothing.
func percent(cur, max int) int {
	if max <= 0 {
		return 0
	}
	return cur * 100 / max
}
