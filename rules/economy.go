package rules

import "fmt"

// Economy is what things cost, from content/rules/economy.json.
//
// Prices are derived, not written on every object: what a piece is worth is
// what its category is worth at power 1, times its power. A builder tunes
// what armor is worth once, and a power-12 helm costs twelve times a power-1
// one without anyone keeping a second number in step -- the same reasoning
// as armor.json and durability.json.
//
// An absent economy.json is the economy switched off: no coins drop, and
// everything is worth nothing, so repair is free and a store gives its stock
// away. That is a game you can play; a half-configured one is not, so the
// loader checks a present one.
type Economy struct {
	// CoinsPerPower is what a mob carries, per point of its power: from that
	// many to twice that many, rolled when it dies. Zero is mobs that carry
	// nothing.
	CoinsPerPower int `json:"coins_per_power"`

	// Value is what a power-1 piece of each category is worth. Default covers
	// a category the table doesn't name.
	Value struct {
		Default    int                    `json:"default"`
		ByCategory map[ObjectCategory]int `json:"by_category"`
	} `json:"value"`

	// SellPercent is what a store pays for something, as a percentage of
	// its price.
	SellPercent int `json:"sell_percent"`

	// RepairPercent is what mending something from broken to new costs, as a
	// percentage of its price. Less worn costs proportionally less.
	RepairPercent int `json:"repair_percent"`
}

// Price is what a piece of this category and power costs to buy. Power 0 is
// priced as power 1: broken-down starter gear is still worth something.
func (e Economy) Price(cat ObjectCategory, power int) int {
	value, listed := e.Value.ByCategory[cat]
	if !listed {
		value = e.Value.Default
	}
	return value * max(power, 1)
}

// SellPrice is what a store pays for a piece it would charge price for: at
// least a coin, when the economy is on at all, so nothing is worth carrying
// back to town for nothing.
func (e Economy) SellPrice(price int) int {
	if price <= 0 || e.SellPercent <= 0 {
		return 0
	}
	return max(1, price*e.SellPercent/100)
}

// RepairCost is what mending missing points of maxDurability costs on a
// piece worth price. At least a coin for any repair at all, for the same
// reason as SellPrice.
func (e Economy) RepairCost(price, missing, maxDurability int) int {
	if price <= 0 || e.RepairPercent <= 0 || missing <= 0 || maxDurability <= 0 {
		return 0
	}
	return max(1, price*e.RepairPercent*missing/(100*maxDurability))
}

// Coins is what a mob of this power carries, given roll in [0, power *
// CoinsPerPower]: between CoinsPerPower and twice that, per point of power.
func (e Economy) Coins(power, roll int) int {
	return max(power, 1)*e.CoinsPerPower + roll
}

// CoinRange is the span Coins' roll is drawn from: IntN(CoinRange) for a
// mob of this power. Zero when mobs carry nothing.
func (e Economy) CoinRange(power int) int {
	if e.CoinsPerPower <= 0 {
		return 0
	}
	return max(power, 1)*e.CoinsPerPower + 1
}

// Check refuses an economy that can't work. Selling has to pay less than
// buying costs, or a store is a machine for making coins out of nothing.
func (e Economy) Check() error {
	switch {
	case e.CoinsPerPower < 0 || e.Value.Default < 0 || e.RepairPercent < 0 || e.SellPercent < 0:
		return fmt.Errorf("economy: nothing can be negative")
	case e.SellPercent >= 100:
		return fmt.Errorf("economy: sell_percent %d would let a store be bought from and sold back to at a profit", e.SellPercent)
	}
	for cat, v := range e.Value.ByCategory {
		if v < 0 {
			return fmt.Errorf("economy: %s is worth %d", cat, v)
		}
	}
	return nil
}
