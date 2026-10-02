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
}

func (a *Ability) UnmarshalJSON(data []byte) error {
	var raw struct {
		Id       string        `json:"id"`
		Name     string        `json:"name"`
		Mana     int           `json:"mana"`
		Cooldown string        `json:"cooldown"`
		Target   AbilityTarget `json:"target"`
		Amount   AbilityAmount `json:"amount"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*a = Ability{
		Id:     raw.Id,
		Name:   raw.Name,
		Mana:   raw.Mana,
		Target: raw.Target,
		Amount: raw.Amount,
	}
	if raw.Cooldown != "" {
		d, err := time.ParseDuration(raw.Cooldown)
		if err != nil {
			return fmt.Errorf("ability %q cooldown: %w", raw.Id, err)
		}
		a.Cooldown = d
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
