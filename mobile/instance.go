package mobile

import (
	"errors"
	"fmt"
	"math/rand"
	"time"
	"uuid"

	"github.com/watchmud/watchmud/combat"
)

// Instance is a Mobile standing in front of you, representing
// its definition. ex. Mobile of definition 'lizard' are all
// immune to poison, but this instance of 'lizard' is wearing
// a magic hat and has a sword in its hand. (scary lizard)
type Instance struct {
	id                uuid.UUID
	Definition        *Definition
	LastWanderingTime time.Time // when was the last time this mob went wandering?
	WanderingForward  bool      // do you wander forward on the path or backwards?
	CurHealth         int
}

func NewInstance(d *Definition) *Instance {
	return &Instance{
		id:                uuid.New(),
		Definition:        d,
		LastWanderingTime: time.Now(),
		WanderingForward:  true, // by default
		CurHealth:         d.MaxHealth,
	}
}

func (mob *Instance) Id() uuid.UUID {
	return mob.id
}

func (mob *Instance) IdStr() string {
	return mob.id.String()
}

func (mob *Instance) Name() string {
	return mob.Definition.Name
}

func (mob *Instance) CanWander() bool {
	return mob.canWander(time.Now())
}

func (mob *Instance) canWander(now time.Time) bool {
	if !mob.Definition.Wandering.CanWander {
		return false
	}
	timeSince := now.Sub(mob.LastWanderingTime)
	return timeSince > mob.Definition.Wandering.CheckFrequency
}

func (mob *Instance) CheckWanderChance() bool {
	return mob.checkWanderChance(rand.New(rand.NewSource(time.Now().UnixNano())))
}

func (mob *Instance) checkWanderChance(r *rand.Rand) bool {
	chance := r.Float32()
	//log.Printf("mob '%s' chance of walking %f vs. %f", mob.Definition.Id, chance, mob.Definition.Wandering.CheckPercentage)
	return chance < mob.Definition.Wandering.CheckPercentage
}

// GetIndexOnPath determines where we are on the wandering path given the current room id.
// returns error if we're not wandering on a path
func (mob *Instance) GetIndexOnPath(currentRoom string) (int, error) {
	if len(mob.Definition.Wandering.Path) == 0 {
		return -1, errors.New("instance not defined to be on a path")
	}
	for i, s := range mob.Definition.Wandering.Path {
		if s == currentRoom {
			return i, nil
		}
	}
	return -1, errors.New(fmt.Sprintf("currentRoom '%s' not found in path '%s'", currentRoom, mob.Definition.Wandering.Path))
}

// Combatant
func (mob *Instance) TakeMeleeDamage(damage int) bool {
	mob.CurHealth = mob.CurHealth - damage
	return mob.CurHealth <= 0
}

// Combatant
func (mob *Instance) Dead() bool {
	return mob.CurHealth <= 0
}

// Combatant. Mobs wear nothing, so a mob's power is its definition's.
func (mob *Instance) Power() int {
	return mob.Definition.Power
}

// Combatant
func (mob *Instance) CalculateMeleeRollModifiers() int {
	// TODO mob melee modifiers come from definition and ...?
	return 0
}

// Combatant
func (mob *Instance) ArmorClass() int {
	// base +/- equipment
	// TODO
	return mob.Definition.ArmorClass()
}

// Combatant
func (mob *Instance) HasResistanceTo(damageType combat.DamageType) bool {
	// resistance gained by spells or equipment?
	// TODO
	return mob.Definition.HasResistanceTo(damageType)
}

// Combatant
func (mob *Instance) IsVulnerableTo(damageType combat.DamageType) bool {
	// vulnerability gained by spells or equipment?
	// TODO
	return mob.Definition.IsVulnerableTo(damageType)
}

// Combatant
func (mob *Instance) WeaponDamageRoll() string {
	return string(mob.Definition.Damage)
}

// Combatant
func (mob *Instance) WeaponDamageType() combat.DamageType {
	// TODO mobs can't carry anything yet
	return combat.Piercing
}

func (mob *Instance) Send(msg any) {
	// TODO do something with this notification
}

func (mob *Instance) Matches(target string) bool {
	return mob.Definition.Matches(target)
}

func (mob *Instance) RestoreMaxHealth() {
	mob.CurHealth = mob.Definition.MaxHealth
}

func (mob *Instance) RestoreHealth(amount int) {
	mob.CurHealth = min(mob.CurHealth+amount, mob.Definition.MaxHealth)
}
