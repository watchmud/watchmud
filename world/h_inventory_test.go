package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

type handleInventorySuite struct {
	worldTestSuite
}

func TestHandleInventorySuite(t *testing.T) {
	suite.Run(t, new(handleInventorySuite))
}
func (s *handleInventorySuite) SetupTest() {
	s.worldTestSuite.SetupTest()
}

func (s *handleInventorySuite) TestInventory_Success() {
	defnPtr := object.NewDefinition(
		"defnid",
		"name",
		"zone",
		rules.ObjectCategoryTreasure,
		[]string{},
		"short desc",
		"in room",
		rules.SlotNone,
		rules.ArmorTypePlate,
		rules.EmptyObjectBehaviors)
	instPtr := &object.Instance{
		Id:         uuid.New(),
		Definition: defnPtr,
	}
	s.p.Inventory().Add(instPtr)

	cmd := command.Inventory{}
	s.w.handleInventory(s.handlerParameter(cmd), cmd)

	s.Assert().Equal(1, len(s.r.Sent))
	resp := s.r.Sent[0].(event.Inventory)
	s.Assert().Equal(1, len(resp.Items))
	s.Assert().Equal(instPtr.Id.String(), resp.Items[0].Id)
	s.Assert().Equal("short desc", resp.Items[0].ShortDescription)
}
