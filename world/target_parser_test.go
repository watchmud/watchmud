package world

import (
	"iter"
	"slices"
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

type TargetParserSuite struct {
	suite.Suite
}

func TestTargetParserSuite(t *testing.T) {
	suite.Run(t, new(TargetParserSuite))
}

func (suite *TargetParserSuite) TestParse() {
	target, err := parseTarget("foo")
	suite.Assert().NoError(err)
	suite.Assert().Equal("foo", target.Name)
	suite.Assert().Equal(0, target.Identifier)
	suite.Assert().Equal(0, target.Quantity)
}

func (suite *TargetParserSuite) TestParseAll() {
	target, err := parseTarget("all")
	suite.Assert().NoError(err)
	suite.Assert().True(target.All)
	suite.Assert().Equal("", target.Name)
	suite.Assert().Equal(0, target.Identifier)
	suite.Assert().Equal(0, target.Quantity)
}

func (suite *TargetParserSuite) TestParseWithIdentifier() {
	target, err := parseTarget("2.foo")
	suite.Assert().NoError(err)
	suite.Assert().Equal("foo", target.Name)
	suite.Assert().Equal(2, target.Identifier)
	suite.Assert().Equal(0, target.Quantity)
}
func (suite *TargetParserSuite) TestParseWithAllIdentifier() {
	target, err := parseTarget("all.foo")
	suite.Assert().NoError(err)
	suite.Assert().Equal("foo", target.Name)
	suite.Assert().True(target.All)
	suite.Assert().Equal(0, target.Identifier)
	suite.Assert().Equal(0, target.Quantity)
}
func (suite *TargetParserSuite) TestParseQuantity() {
	target, err := parseTarget("50 foo")
	suite.Assert().NoError(err)
	suite.Assert().Equal("foo", target.Name)
	suite.Assert().Equal(0, target.Identifier)
	suite.Assert().Equal(50, target.Quantity)
}

func (suite *TargetParserSuite) TestParseQuantityNotNumber() {
	_, err := parseTarget("x foo")
	suite.Assert().Error(err)
}

func (suite *TargetParserSuite) TestParseQuantityWithIdentifier() {
	target, err := parseTarget("50 2.foo")
	suite.Assert().NoError(err)
	suite.Assert().Equal("foo", target.Name)
	suite.Assert().Equal(2, target.Identifier)
	suite.Assert().Equal(50, target.Quantity)
}

func (suite *TargetParserSuite) TestParseEmpty() {
	target, err := parseTarget("")
	suite.Assert().NoError(err)
	suite.Assert().Equal("", target.Name)
	suite.Assert().Equal(0, target.Identifier)
	suite.Assert().Equal(0, target.Quantity)
}

func (suite *TargetParserSuite) TestParseTooManyParts() {
	_, err := parseTarget("foo bar baz")
	suite.Assert().Error(err)
}

func (suite *TargetParserSuite) TestParseTooManyDots() {
	_, err := parseTarget("50 5.bar.baz")
	suite.Assert().Error(err)
}

// what the grammar picks out of a container, in the order things arrived
func (suite *TargetParserSuite) TestTargetsIn() {
	knifeDef := object.NewDefinition(
		"knife",
		"knife",
		"zone",
		rules.ObjectCategoryWeapon,
		[]string{"blade"},
		"knife",
		"A knife is here.",
		rules.SlotWield,
		rules.ArmorTypeNone,
		rules.EmptyObjectBehaviors)

	helmDef := object.NewDefinition(
		"helm",
		"helmet",
		"zone",
		rules.ObjectCategoryArmor,
		nil,
		"helmet",
		"A helmet is here.",
		rules.SlotHead,
		rules.ArmorTypePlate,
		rules.EmptyObjectBehaviors,
	)

	first := object.NewInstance(uuid.New(), knifeDef)
	helm := object.NewInstance(uuid.New(), helmDef)
	second := object.NewInstance(uuid.New(), knifeDef)
	contents := func() iter.Seq[*object.Instance] {
		return slices.Values([]*object.Instance{first, helm, second})
	}

	cases := []struct {
		target string
		want   []*object.Instance
	}{
		{"knife", []*object.Instance{first}},    // the first one
		{"1.knife", []*object.Instance{first}},  // same thing said explicitly
		{"2.knife", []*object.Instance{second}}, // the second one
		{"3.knife", nil},                        // there is no third
		{"blade", []*object.Instance{first}},    // aliases count
		{"all.knife", []*object.Instance{first, second}},
		{"all", []*object.Instance{first, helm, second}}, // everything, in arrival order
		{"sword", nil}, // nothing by that name
		{"all.sword", nil},
		{"2.helmet", nil}, // only one of those
	}
	for _, tc := range cases {
		suite.Run(tc.target, func() {
			target, err := parseTarget(tc.target)
			suite.Require().NoError(err)
			suite.Assert().Equal(tc.want, targetsIn(target, contents()))
		})
	}
}

func (suite *TargetParserSuite) TestTargetsIn_EmptyContainer() {
	empty := func(yield func(*object.Instance) bool) {}

	target, err := parseTarget("all")
	suite.Require().NoError(err)
	suite.Assert().Empty(targetsIn(target, empty))

	target, err = parseTarget("knife")
	suite.Require().NoError(err)
	suite.Assert().Empty(targetsIn(target, empty))
}

// What would act on the wrong thing, or panic, is refused instead: an index
// before the first, a number or "all." with no name after it (a half-typed
// "junk 2." mustn't junk whatever's second), and a count of none -- "put 0
// coins" isn't "put coins", which is all of them.
func (suite *TargetParserSuite) TestParseRefuses() {
	for _, bad := range []string{"-1.knife", "0.knife", "2.", "all.", "0 coins", "-5 coins"} {
		_, err := parseTarget(bad)
		suite.Assert().Error(err, bad)
	}
}

// and from the player's side: get -1.knife used to panic
func (suite *TargetParserSuite) TestNegativeIndexDoesntPanic() {
	w, err := NewTestWorld()
	suite.Require().NoError(err)
	r := &player.Recorder{}
	p := player.NewTestPlayer(uuid.New(), "dood", r)
	w.PlacePlayer(p, w.StartRoom)
	suite.Require().NotPanics(func() {
		suite.Require().NoError(w.HandleIncomingMessage(gameserver.NewHandlerParameter(gameserver.NewTestConn(p), command.Get{Target: "-1.knife"})))
	})
	suite.Assert().Equal(event.ParseError, sent[event.Failed](suite.T(), r, len(r.Sent)-1).Code)
}
