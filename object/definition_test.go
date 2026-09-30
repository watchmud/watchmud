package object

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/rules"
)

type DefinitionSuite struct {
	suite.Suite
	helmet *Instance
}

func TestDefinitionSuite(t *testing.T) {
	suite.Run(t, new(DefinitionSuite))
}

func (s *DefinitionSuite) SetupTest() {
	s.helmet = MakeTestArmor(s.T(), rules.SlotHead, "helmet", rules.ArmorTypePlate, 100)
	s.helmet.Definition.Aliases = []string{"helm"}
}

func (s *DefinitionSuite) TestHasAlias() {
	s.Assert().True(s.helmet.Definition.HasAlias("helm"), "should have alias")
	s.Assert().False(s.helmet.Definition.HasAlias("bronze"), "should not have alias")
}
