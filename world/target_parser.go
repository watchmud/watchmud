package world

import (
	"errors"
	"iter"
	"strconv"
	"strings"

	"github.com/watchmud/watchmud/object"
)

type Target struct {
	Quantity   int    //  "2000 coins" = 2000
	Identifier int    // "2.knife" = 2
	Name       string // "knife" = "knife"
	All        bool
}

// Take the target string and parse it into Target struct
// Example usage:
//
//	   drop <item>
//	   drop all.<item>
//	   drop all
//	   drop <number> coins
//		drop 2.<item>
func parseTarget(target string) (result Target, err error) {

	parts := strings.Split(target, " ")

	if len(parts) > 2 {
		err = errors.New("TOO_MANY_PARTS_IN_TARGET")
		return
	}
	var item string
	if len(parts) == 2 {
		// "x xyz"
		var quant int
		quant, err = strconv.Atoi(parts[0])
		if err != nil {
			return
		}
		// "0 coins" isn't "coins", which means all of them
		if quant < 1 {
			err = errors.New("QUANTITY_BELOW_ONE")
			return
		}
		result.Quantity = quant
		item = parts[1]
	} else {
		// "xyz"
		item = parts[0]
	}

	dotparts := strings.Split(item, ".")
	if len(dotparts) == 1 {
		if strings.EqualFold(item, "all") {
			// "all"
			result.All = true
		} else {
			// "foo"
			result.Identifier = 0
			result.Name = item
		}
	} else if len(dotparts) == 2 {
		// "x.foo"
		// "all." and "2." name nothing; only bare "all" means everything, and
		// a line sent half-typed mustn't junk whatever is second in the pack
		if dotparts[1] == "" {
			err = errors.New("NOTHING_AFTER_THE_DOT")
			return
		}
		if strings.EqualFold(dotparts[0], "all") {
			// "all.foo"
			result.All = true
			result.Name = dotparts[1]
		} else {
			// "4.foo"
			var identifier int
			identifier, err = strconv.Atoi(dotparts[0])
			if err != nil {
				return
			}
			// counted from 1: "-1.knife" would index before the start
			if identifier < 1 {
				err = errors.New("INDEX_BELOW_ONE")
				return
			}
			result.Identifier = identifier
			result.Name = dotparts[1]
		}
	} else {
		// "x.x.x"
		err = errors.New("TOO_MANY_DOTS")
	}
	return
}

// targetsIn picks the instances a parsed Target names out of a container's
// contents, which arrive in the order they were put there:
//
//	knife      the first one
//	2.knife    the second one
//	all.knife  every one of them
//	all        everything in there
//
// An index past the end selects nothing, the same as a name nothing matches:
// "you don't see that here" is the right answer to both.
func targetsIn(t Target, contents iter.Seq[*object.Instance]) []*object.Instance {
	var matches []*object.Instance
	for inst := range contents {
		// an empty name is bare "all", which names everything
		if t.Name == "" || inst.Matches(t.Name) {
			matches = append(matches, inst)
		}
	}
	if t.All {
		return matches
	}
	// "knife" is "1.knife"
	n := t.Identifier
	if n == 0 {
		n = 1
	}
	if n < 1 || n > len(matches) {
		return nil
	}
	return matches[n-1 : n]
}
