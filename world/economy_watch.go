package world

import (
	"github.com/rs/zerolog/log"
)

// EconomyReport is the numbers inflation shows up in long before players say
// so (ROADMAP, "Economy inflation"): the coins people carry, and what lies
// free in the donation room. People and bots apart: bots hunt all day and
// spend nothing, and would hide what players are doing.
type EconomyReport struct {
	People, Bots          int
	PeopleCoins, BotCoins int
	// MostCoins is the richest person playing.
	MostCoins int
	// Donated is what's on the donation room's floor, and the mean and top
	// power of it.
	Donated          int
	DonatedMeanPower float64
	DonatedTopPower  int
}

// Economy counts the coins of everyone playing and what's in the donation
// room. Only who's online: the store isn't asked, as the world goroutine
// never waits on it.
func (w *World) Economy() EconomyReport {
	var r EconomyReport
	for p := range w.Players() {
		if p.IsBot() {
			r.Bots++
			r.BotCoins += p.Coins()
			continue
		}
		r.People++
		r.PeopleCoins += p.Coins()
		r.MostCoins = max(r.MostCoins, p.Coins())
	}
	d := w.content.Settings.Donation
	if donation, found := w.findRoomById(d.ZoneId, d.RoomId); found {
		total := 0
		for inst := range donation.Inventory.All() {
			r.Donated++
			total += inst.Power
			r.DonatedTopPower = max(r.DonatedTopPower, inst.Power)
		}
		if r.Donated > 0 {
			r.DonatedMeanPower = float64(total) / float64(r.Donated)
		}
	}
	return r
}

// LogEconomy writes Economy to the log, for the heartbeat.
func (w *World) LogEconomy() {
	r := w.Economy()
	log.Info().
		Int("people", r.People).Int("peopleCoins", r.PeopleCoins).Int("mostCoins", r.MostCoins).
		Int("bots", r.Bots).Int("botCoins", r.BotCoins).
		Int("donated", r.Donated).Float64("donatedMeanPower", r.DonatedMeanPower).Int("donatedTopPower", r.DonatedTopPower).
		Msg("economy")
}
