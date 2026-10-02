package rules

import (
	"encoding/json"
	"fmt"
	"time"
)

type MudTime struct {
	// Mobiles determine their activity at this duration
	Mobile time.Duration `json:"mobile"`

	// Violence determines the basic speed of combat
	Violence time.Duration `json:"violence"`

	// Zone reset times are evaluated at this duration
	Zone time.Duration `json:"zone"`

	// PlayerSave is how long between we queue up save operations
	PlayerSave time.Duration `json:"playerSave"`

	// Regen is how often anyone not fighting gets some health back, and
	// every player some mana
	Regen time.Duration `json:"regen"`
}

func (m *MudTime) UnmarshalJSON(data []byte) error {
	var raw struct {
		Mobile     string `json:"mobile"`
		Violence   string `json:"violence"`
		Zone       string `json:"zone"`
		PlayerSave string `json:"playerSave"`
		Regen      string `json:"regen"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var err error
	if m.Mobile, err = parseInterval("mobile", raw.Mobile); err != nil {
		return err
	}
	if m.Violence, err = parseInterval("violence", raw.Violence); err != nil {
		return err
	}
	if m.Zone, err = parseInterval("zone", raw.Zone); err != nil {
		return err
	}
	if m.PlayerSave, err = parseInterval("playerSave", raw.PlayerSave); err != nil {
		return err
	}
	if m.Regen, err = parseInterval("regen", raw.Regen); err != nil {
		return err
	}
	return nil
}

func parseInterval(name, s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("mudtime.json %s: %w", name, err)
	}
	if d <= 0 || d%PulseInterval != 0 {
		return 0, fmt.Errorf("mudtime.json %s: %s is not a whole number of pulses (%s each)", name, d, PulseInterval)
	}
	return d, nil
}
