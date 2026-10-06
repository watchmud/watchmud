package rules

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Ability struct {
	Id       string
	Name     string
	Mana     int
	Cooldown time.Duration
	Target   AbilityTarget
	Amount   AbilityAmount
	// Duration is how long what it leaves behind lasts: ward's shield. Zero
	// for an ability that is over when it's cast.
	Duration time.Duration
	// NotInFight refuses a cast mid-fight, before anything is spent: recall,
	// which would otherwise be a free, certain escape and make flee pointless.
	NotInFight bool
	// Wizards is an ability a wizard always has: no gear granting it and no
	// cooldown. Recall, which a builder needs to get about.
	Wizards bool
}

func (a *Ability) UnmarshalJSON(data []byte) error {
	var raw struct {
		Id         string        `json:"id"`
		Name       string        `json:"name"`
		Mana       int           `json:"mana"`
		Cooldown   string        `json:"cooldown"`
		Target     AbilityTarget `json:"target"`
		Amount     AbilityAmount `json:"amount"`
		Duration   string        `json:"duration"`
		NotInFight bool          `json:"not_in_fight"`
		Wizards    bool          `json:"wizards"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*a = Ability{
		Id:         raw.Id,
		Name:       raw.Name,
		Mana:       raw.Mana,
		Target:     raw.Target,
		Amount:     raw.Amount,
		NotInFight: raw.NotInFight,
		Wizards:    raw.Wizards,
	}
	if raw.Cooldown != "" {
		d, err := time.ParseDuration(raw.Cooldown)
		if err != nil {
			return fmt.Errorf("ability %q cooldown: %w", raw.Id, err)
		}
		a.Cooldown = d
	}
	if raw.Duration != "" {
		d, err := time.ParseDuration(raw.Duration)
		if err != nil {
			return fmt.Errorf("ability %q duration: %w", raw.Id, err)
		}
		a.Duration = d
	}
	return nil
}

func (a *Ability) check() error {
	switch {
	case a.Id == "":
		return fmt.Errorf("ability %q: missing id", a.Name)
	case a.Name == "":
		return fmt.Errorf("ability %q: missing name", a.Id)
	case a.Mana < 0:
		return fmt.Errorf("ability %q: mana %d", a.Id, a.Mana)
	case a.Cooldown < 0:
		return fmt.Errorf("ability %q: cooldown %s", a.Id, a.Cooldown)
	case a.Duration < 0:
		return fmt.Errorf("ability %q: duration %s", a.Id, a.Duration)
	case !a.Target.valid():
		return fmt.Errorf("ability %q: unknown target %q", a.Id, a.Target)
	}
	return nil
}

type AbilityTarget string

const (
	TargetNone   AbilityTarget = "none"   // no target
	TargetSelf   AbilityTarget = "self"   // always the caster
	TargetFriend AbilityTarget = "friend" // the caster or a player in their room
	TargetFoe    AbilityTarget = "foe"    // a mob in the caster's room
	// TargetMob is a mob in the caster's room too, but never refused for
	// where it is or what it is: looking at something isn't fighting it.
	TargetMob AbilityTarget = "mob"
)

func (t AbilityTarget) valid() bool {
	_, err := ParseAbilityTarget(string(t))
	return err == nil
}

var ErrUnknownAbilityTarget = errors.New("unknown ability target")

var abilityTargets = enum[AbilityTarget]{
	"ability target",
	ErrUnknownAbilityTarget,
	[]AbilityTarget{
		TargetNone,
		TargetSelf,
		TargetFriend,
		TargetFoe,
		TargetMob,
	},
}

func ParseAbilityTarget(s string) (AbilityTarget, error) {
	return abilityTargets.parse(s)
}

func (t *AbilityTarget) UnmarshalText(b []byte) error {
	return abilityTargets.unmarshal(t, b)
}

type AbilityAmount struct {
	Base     int `json:"base"`
	PerPower int `json:"per_power"`
}

// For is the amount at this power - the power of the item granting it.
func (a AbilityAmount) For(power int) int {
	return a.Base + a.PerPower*power
}
