package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testEconomy() Economy {
	var e Economy
	e.CoinsPerPower = 3
	e.Value.Default = 10
	e.Value.ByCategory = map[ObjectCategory]int{ObjectCategoryArmor: 20, ObjectCategoryTreasure: 4}
	e.SellPercent = 40
	e.RepairPercent = 50
	return e
}

func TestEconomy_price(t *testing.T) {
	e := testEconomy()
	assert.Equal(t, 20, e.Price(ObjectCategoryArmor, 1))
	assert.Equal(t, 60, e.Price(ObjectCategoryArmor, 3), "power multiplies it")
	assert.Equal(t, 20, e.Price(ObjectCategoryArmor, 0), "power 0 is priced as 1")
	assert.Equal(t, 10, e.Price(ObjectCategoryFood, 1), "an unlisted category is the default")
}

func TestEconomy_sellPrice(t *testing.T) {
	e := testEconomy()
	assert.Equal(t, 8, e.SellPrice(20))
	assert.Equal(t, 1, e.SellPrice(4), "never nothing for something")
	assert.Equal(t, 0, e.SellPrice(0))
	assert.Equal(t, 0, Economy{}.SellPrice(20), "no economy, no market")
}

func TestEconomy_repairCost(t *testing.T) {
	e := testEconomy()
	assert.Equal(t, 10, e.RepairCost(20, 40, 40), "broken to new: half the price")
	assert.Equal(t, 5, e.RepairCost(20, 20, 40), "half worn: half that")
	assert.Equal(t, 1, e.RepairCost(20, 1, 40), "a scratch still costs a coin")
	assert.Equal(t, 0, e.RepairCost(20, 0, 40), "nothing to mend")
	assert.Equal(t, 0, Economy{}.RepairCost(20, 40, 40), "no economy: free")
}

func TestEconomy_coins(t *testing.T) {
	e := testEconomy()
	assert.Equal(t, 7, e.CoinRange(2), "a power-2 mob: 6 to 12")
	assert.Equal(t, 6, e.Coins(2, 0))
	assert.Equal(t, 12, e.Coins(2, 6))
	assert.Equal(t, 0, Economy{}.CoinRange(5), "no economy, no coins")
}

func TestEconomy_check(t *testing.T) {
	assert.NoError(t, testEconomy().Check())
	assert.NoError(t, Economy{}.Check(), "switched off is fine")

	e := testEconomy()
	e.SellPercent = 100
	assert.Error(t, e.Check(), "buy, sell back, repeat")

	e = testEconomy()
	e.Value.ByCategory[ObjectCategoryWeapon] = -1
	assert.Error(t, e.Check())
}
