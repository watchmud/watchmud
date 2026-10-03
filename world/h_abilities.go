package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

// handleAbilities lists what the players gear lets them cast, in the order
// the catalog declares them, so the list doesn't reshuffle between asks.
func (w *World) handleAbilities(msg *gameserver.HandlerParameter, _ command.Abilities) {
	p := msg.Player
	grants := p.Equipment().Abilities()
	now := w.now()
	var listed []event.GrantedAbility
	for _, a := range w.content.Catalog.AbilityList() {
		g, granted := grants[a.Id]
		if !granted {
			continue
		}
		listed = append(listed, event.GrantedAbility{
			Name:     a.Name,
			Mana:     a.Mana,
			Cooldown: a.Cooldown,
			Item:     g.Instance.Definition.ShortDescription,
			Power:    g.Power,
			ReadyIn:  max(p.ReadyAt(a.Id).Sub(now), 0),
		})
	}
	p.Send(event.Abilities{Granted: listed})
}
