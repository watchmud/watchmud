// Package sim fights players against mobs, many times over, through the same
// combat code the game runs, to say how a fight goes before anybody plays it:
// how often the player wins, how long it takes, and what it costs them.
//
// It is melee and nothing else. Both sides swing once a round, the player
// first, as a kill opens a fight -- combat.AttemptMeleeAttack and
// TakeMeleeDamage, exactly as World.DoViolence calls them. What it leaves out
// is said here so a number isn't read for more than it is: abilities (smite,
// heal, ward, stun, provoke), scripts (a boss's summons), gear wearing down
// mid-fight, regeneration, fleeing and more than one on either side.
package sim

import (
	"fmt"
	"strings"
	"uuid"

	"github.com/watchmud/watchmud/combat"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

// MaxRounds is where a fight is called a draw: neither side can hurt the
// other, or not fast enough to matter.
const MaxRounds = 500

// Kit is what the simulated player wears: object definitions, every one
// made at one power and worn.
type Kit []*object.Definition

// StartingKit is what a new character is created wearing.
func StartingKit(c *loader.Content) (Kit, error) {
	var refs []string
	for _, item := range c.Catalog.StartingGear {
		if item.Equip {
			refs = append(refs, item.ZoneId+"/"+item.DefinitionId)
		}
	}
	return KitOf(c, refs)
}

// KitOf is the objects refs names, each "zone/id"; each must be wearable, and
// no two may want one slot.
func KitOf(c *loader.Content, refs []string) (Kit, error) {
	var kit Kit
	slots := map[rules.EquipmentSlot]string{}
	for _, ref := range refs {
		zoneId, id, ok := strings.Cut(ref, "/")
		if !ok {
			return nil, fmt.Errorf("%q: want zone/id", ref)
		}
		z := c.Zones[zoneId]
		if z == nil {
			return nil, fmt.Errorf("%q: no zone %s", ref, zoneId)
		}
		d := z.ObjectDefinitions[id]
		if d == nil {
			return nil, fmt.Errorf("%q: no object %s in %s", ref, id, zoneId)
		}
		if d.EquipmentSlot == rules.SlotNone || d.EquipmentSlot == "" {
			return nil, fmt.Errorf("%q: nothing wears it", ref)
		}
		if other, taken := slots[d.EquipmentSlot]; taken {
			return nil, fmt.Errorf("%q and %q both go in %s", other, ref, d.EquipmentSlot)
		}
		slots[d.EquipmentSlot] = ref
		kit = append(kit, d)
	}
	return kit, nil
}

// Player is a fresh character wearing kit at power, at full health.
func (k Kit) Player(c *loader.Content, power int) *player.Player {
	p := player.New(uuid.New(), "Sim", "", nobody{}, c.Catalog.DefaultLineage(), c.Catalog)
	for _, d := range k {
		inst := object.NewInstance(uuid.New(), d)
		inst.Power = power
		_ = p.Inventory().Add(inst)
		p.Equipment().Equip(d.EquipmentSlot, inst)
	}
	return p
}

// nobody is where a simulated player's messages go.
type nobody struct{}

func (nobody) Send(any) {}

// Outcome is one fight.
type Outcome struct {
	Won, Draw  bool
	Rounds     int
	HealthLeft int // the player's, 0 on a loss
	MaxHealth  int
}

// Fight is one fight to a death, or a draw at MaxRounds.
func Fight(roller rules.Roller, p *player.Player, mob *mobile.Instance) (Outcome, error) {
	o := Outcome{MaxHealth: p.MaxHealth()}
	for o.Rounds = 1; o.Rounds <= MaxRounds; o.Rounds++ {
		for _, pair := range [2][2]combat.Combatant{{p, mob}, {mob, p}} {
			hit, err := combat.AttemptMeleeAttack(roller, pair[0], pair[1])
			if err != nil {
				return o, err
			}
			if hit.WasHit && pair[1].TakeMeleeDamage(hit.Damage) {
				o.Won = pair[1] == combat.Combatant(mob)
				if o.Won {
					o.HealthLeft = p.CurrentHealth()
				}
				return o, nil
			}
		}
	}
	o.Rounds = MaxRounds
	o.Draw = true
	o.HealthLeft = p.CurrentHealth()
	return o, nil
}

// Summary is many fights of one matchup.
type Summary struct {
	Fights, Wins, Draws int
	rounds, healthLeft  int // sums, over every fight and every win
}

// WinRate is the share of fights the player won, 0 to 1.
func (s Summary) WinRate() float64 { return ratio(s.Wins, s.Fights) }

// Rounds is how long a fight lasts, on average, won or lost.
func (s Summary) Rounds() float64 { return ratio(s.rounds, s.Fights) }

// HealthLeft is the share of their health a winner walks away with.
func (s Summary) HealthLeft() float64 { return ratio(s.healthLeft, s.Wins*100) }

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

// Run is n fights of kit at power against fresh copies of def.
func Run(roller rules.Roller, c *loader.Content, kit Kit, power int, def *mobile.Definition, n int) (Summary, error) {
	var s Summary
	for range n {
		p := kit.Player(c, power)
		o, err := Fight(roller, p, mobile.NewInstance(def))
		if err != nil {
			return s, err
		}
		s.Fights++
		s.rounds += o.Rounds
		switch {
		case o.Won:
			s.Wins++
			s.healthLeft += o.HealthLeft * 100 / o.MaxHealth
		case o.Draw:
			s.Draws++
		}
	}
	return s, nil
}
