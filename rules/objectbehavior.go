package rules

import (
	"errors"
)

type ObjectBehavior string

const (
	ObjectBehaviorNone   ObjectBehavior = ""
	ObjectBehaviorNoTake ObjectBehavior = "noTake"
	// NoSell is what no shop buys: a key a reset puts back every few minutes
	// would otherwise be a coin farm.
	ObjectBehaviorNoSell ObjectBehavior = "noSell"
)

var ErrUnknownObjectBehavior = errors.New("unknown object behavior")

var objectBehaviors = enum[ObjectBehavior]{
	"object behavior",
	ErrUnknownObjectBehavior,
	[]ObjectBehavior{
		ObjectBehaviorNone,
		ObjectBehaviorNoTake,
		ObjectBehaviorNoSell,
	},
}

var EmptyObjectBehaviors []ObjectBehavior

func ParseObjectBehavior(s string) (ObjectBehavior, error) {
	return objectBehaviors.parse(s)
}

func (c *ObjectBehavior) UnmarshalText(b []byte) error {
	return objectBehaviors.unmarshal(c, b)
}
