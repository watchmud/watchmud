package world

import (
	"uuid"

	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

// tradeSuite stands in testcontent's General Store, which sells an iron
// helmet at power 2 (armor: 20 a power, so 40) and a knife at power 1
// (weapon: 20). The store pays 40% of a price.
type tradeSuite struct {
	worldTestSuite
}

func (s *tradeSuite) toStore() {
	s.T().Helper()
	store, found := s.w.findRoomById("wrathrock", "general_store")
	s.Require().True(found)
	s.w.movePlayerMagically(s.p, store)
	s.r.Sent = nil
}

func (s *tradeSuite) do(cmd command.Command) {
	s.T().Helper()
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(cmd)))
}

// carry something worth selling: armor at this power, worn this much
func (s *tradeSuite) carry(name string, power, durability, maxDurability int) *object.Instance {
	s.T().Helper()
	d := object.NewTestDefinition(s.T(), name, rules.SlotHead, rules.ObjectCategoryArmor, rules.ArmorTypeLeather)
	d.MaxDurability = maxDurability
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = power
	inst.Durability = durability
	s.Require().NoError(s.p.Inventory().Add(inst))
	return inst
}
