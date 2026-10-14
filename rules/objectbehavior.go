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
	// NoDonate is what can't be sent to the donation room: a boss's own
	// drop, worn by someone who fought for it rather than left for anyone.
	ObjectBehaviorNoDonate ObjectBehavior = "noDonate"
)

var ErrUnknownObjectBehavior = errors.New("unknown object behavior")

var objectBehaviors = enum[ObjectBehavior]{
	"object behavior",
	ErrUnknownObjectBehavior,
	[]ObjectBehavior{
		ObjectBehaviorNone,
		ObjectBehaviorNoTake,
		ObjectBehaviorNoSell,
		ObjectBehaviorNoDonate,
	},
}

var EmptyObjectBehaviors []ObjectBehavior

func ParseObjectBehavior(s string) (ObjectBehavior, error) {
	return objectBehaviors.parse(s)
}

func (c *ObjectBehavior) UnmarshalText(b []byte) error {
	return objectBehaviors.unmarshal(c, b)
}
