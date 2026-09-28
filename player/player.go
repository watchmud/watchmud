package player

import (
	"uuid"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/combat"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

type Player struct {
	id           uuid.UUID
	name         string
	out          Sender
	passwordHash string
	// wizard lets them run the builder commands. Nothing in the game grants
	// it: it is set by hand on the record (make wizard NAME=...).
	wizard bool

	// Lineage is cosmetic and nothing reads it but the renderer. There is no
	// Class beside it and no Role in its place: a role is read off
	// the equipped slots every time it is asked for, never stored.
	Lineage   *rules.Lineage
	inventory *object.List
	equipment *object.Equipment
	curHealth int
	maxHealth int
}

// New player. The catalog goes to the equipment, which needs it to say what
// gear is worth; the player itself holds neither it nor anything derived from
// it.
func New(id uuid.UUID,
	name string,
	passwordHash string,
	out Sender,
	lineage *rules.Lineage,
	cat *rules.Catalog,
) *Player {
	return &Player{
		id:           id,
		name:         name,
		passwordHash: passwordHash,
		out:          out,
		Lineage:      lineage,
		inventory:    object.NewList(),
		equipment:    object.NewEquipment(cat),
		curHealth:    100, // TODO need a default here,
		maxHealth:    100,
	}
}

func (p *Player) Id() uuid.UUID {
	return p.id
}

func (p *Player) Name() string {
	return p.name
}

// Inventory returns the inventory
func (p *Player) Inventory() *object.List {
	return p.inventory
}

func (p *Player) Equipment() *object.Equipment {
	return p.equipment
}

// RoleWeights totals what the player's equipped gear contributes to each
// role. Resolving that to a rules.Role is the caller's job -- see
// rules.Catalog.RoleFor.
func (p *Player) RoleWeights() map[string]int {
	return p.equipment.RoleWeights()
}

// LineageName is the player's lineage for display. Cosmetic, and empty if
// they somehow have none.
func (p *Player) LineageName() string {
	if p.Lineage == nil {
		return ""
	}
	return p.Lineage.Name
}

func (p *Player) CurrentHealth() int {
	return p.curHealth
}

func (p *Player) MaxHealth() int {
	return p.maxHealth
}

func (p *Player) Send(msg any) {
	p.out.Send(msg)
}

func (p *Player) RestoreMaxHealth() {
	p.curHealth = p.maxHealth
}

func (p *Player) RestoreHealth(amount int) {
	p.curHealth = min(p.curHealth+amount, p.maxHealth)
}

// Revive brings a dead player back with a single point of health. Getting
// the rest back is up to them.
func (p *Player) Revive() {
	p.curHealth = 1
}

func (p *Player) Dead() bool {
	return p.curHealth <= 0
}

func (p *Player) ArmorClass() int {
	return p.equipment.ArmorClass()
}

// Power is the average of what's worn, recomputed on every read -- see
// object.Equipment.Power. There is no stored level to disagree with it.
func (p *Player) Power() int {
	return p.equipment.Power()
}

func (p *Player) CalculateMeleeRollModifiers() int {
	return 0
}

func (p *Player) HasResistanceTo(damageType combat.DamageType) bool {
	// TODO
	return false
}

func (p *Player) TakeMeleeDamage(damage int) bool {
	p.curHealth -= damage
	if p.curHealth <= 0 {
		return true
	}
	return false
}

func (p *Player) IsVulnerableTo(damageType combat.DamageType) bool {
	// TODO
	return false
}

// WeaponDamageRoll is the dice for whatever is wielded, or bare hands. A
// broken weapon is bare hands too: broken gear stays worn, and stops counting.
func (p *Player) WeaponDamageRoll() string {
	if w := p.equipment.At(rules.SlotWield); w != nil && !w.Broken() {
		return string(w.Definition.Damage)
	}
	return string(rules.BareHands)
}

func (p *Player) WeaponDamageType() combat.DamageType {
	// TODO
	return combat.Piercing
}

func (p *Player) Log() *zerolog.Logger {
	l := log.Logger.With().
		Str("playerName", p.Name()).
		Str("playerId", p.Id().String()).
		Logger()
	return &l
}

func (p *Player) IsWizard() bool        { return p.wizard }
func (p *Player) SetWizard(wizard bool) { p.wizard = wizard }
