package player

import (
	"time"
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
	// bot marks a character a program plays, so who can say so. Nothing in
	// the game grants it: it is set by hand on the record (make bot NAME=...).
	bot bool
	// noHassle keeps aggressive mobs off a wizard. Only counts alongside
	// wizard, and isn't saved: World.Arrive switches it on for every
	// wizard's session, so protected is the state nobody has to remember.
	noHassle bool
	// noColor is the player turning ANSI color off. Inverted so the zero
	// value, which is every record written before there was a choice, is on.
	noColor bool
	// coins is the purse. Never negative: Spend refuses what it can't pay.
	coins int

	// Lineage is cosmetic and nothing reads it but the renderer. There is no
	// Class beside it and no Role in its place: a role is read off
	// the equipped slots every time it is asked for, never stored.
	Lineage   *rules.Lineage
	inventory *object.List
	equipment *object.Equipment
	curHealth int
	maxHealth int
	curMana   int
	maxMana   int
	// readyAt is when each ability (by id) can be cast again. In memory
	// only: a quit resets it, and logging back in takes longer than any
	// cooldown.
	readyAt map[string]time.Time
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
		curMana:      rules.MaxMana,
		maxMana:      rules.MaxMana,
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

func (p *Player) CurrentMana() int { return p.curMana }

func (p *Player) MaxMana() int { return p.maxMana }

// SpendMana takes n if there is that much, and says whether it did.
func (p *Player) SpendMana(n int) bool {
	if n < 0 || n > p.curMana {
		return false
	}
	p.curMana -= n
	return true
}

func (p *Player) RestoreMana(n int) {
	p.curMana = min(p.curMana+max(n, 0), p.maxMana)
}

// ReadyAt is when this ability can next be cast; the zero time if it
// never has been.
func (p *Player) ReadyAt(abilityId string) time.Time {
	return p.readyAt[abilityId]
}

func (p *Player) StartCooldown(abilityId string, until time.Time) {
	if p.readyAt == nil {
		p.readyAt = make(map[string]time.Time)
	}
	p.readyAt[abilityId] = until
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
func (p *Player) IsBot() bool           { return p.bot }
func (p *Player) SetBot(bot bool)       { p.bot = bot }
func (p *Player) NoHassle() bool        { return p.noHassle }
func (p *Player) SetNoHassle(on bool)   { p.noHassle = on }
func (p *Player) Color() bool           { return !p.noColor }
func (p *Player) Coins() int            { return p.coins }

// AddCoins puts n in the purse.
func (p *Player) AddCoins(n int) { p.coins += max(n, 0) }

// Spend takes n from the purse if it holds that much, and says whether it did.
func (p *Player) Spend(n int) bool {
	if n < 0 || n > p.coins {
		return false
	}
	p.coins -= n
	return true
}
func (p *Player) SetColor(on bool) { p.noColor = !on }
