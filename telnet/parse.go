package telnet

import (
	"errors"
	"strings"

	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/rules"
)

// parseCommand turns a line the player typed into a command.
//
// It is the counterpart to render.go: one chokepoint in, one chokepoint out,
// and no game logic in either.
//
// Target strings are passed through raw -- world.parseTarget owns the "all",
// "all.knife", "2.knife", "20 coins" grammar, and duplicating that in a
// transport is how the two drift apart. An empty target is not an error here
// either; the handler answers NO_TARGET, which is why a bare "drop" no longer
// needs a special case (it used to panic the process).
//
// The returned error is text to show the player, not a fault.
func parseCommand(tokens []string) (command.Command, error) {
	if len(tokens) == 0 {
		return nil, nil
	}
	verb := strings.ToLower(tokens[0])
	rest := strings.Join(tokens[1:], " ")

	switch verb {
	case "quit":
		return command.Logout{Cause: "quit"}, nil

	case "look", "l":
		if rest == "in" || strings.HasPrefix(rest, "in ") {
			return command.Look{Target: strings.TrimSpace(rest[len("in"):]), In: true}, nil
		}
		return command.Look{Target: rest}, nil

	case "n", "north", "s", "south", "e", "east", "w", "west", "u", "up", "d", "down":
		dir, err := rules.ParseDirection(verb)
		if err != nil {
			return nil, errors.New("You can't go that way.")
		}
		return command.Move{Direction: dir}, nil

	case "exits", "exit", "ex":
		return command.Exits{}, nil

	case "recall":
		return command.Recall{}, nil

	case "get":
		// "get knife from corpse", and "get from corpse" with nothing named,
		// which the handler answers the way it answers a bare "get".
		if from, ok := strings.CutPrefix(rest, "from "); ok {
			return command.Get{From: strings.TrimSpace(from)}, nil
		}
		if target, from, ok := strings.Cut(rest, " from "); ok {
			return command.Get{Target: strings.TrimSpace(target), From: strings.TrimSpace(from)}, nil
		}
		return command.Get{Target: rest}, nil

	case "drop":
		return command.Drop{Target: rest}, nil

	case "repair":
		return command.Repair{Target: rest}, nil

	case "list":
		return command.List{}, nil

	case "buy":
		return command.Buy{Target: rest}, nil

	case "sell":
		return command.Sell{Target: rest}, nil

	case "value":
		return command.Value{Target: rest}, nil

	case "remove", "rem", "unwear", "unwield":
		return command.Remove{Target: rest}, nil

	case "wear":
		return command.Wear{Target: rest}, nil

	case "wield":
		return command.Equip{Target: rest, Slot: rules.SlotWield}, nil

	case "inv", "inventory", "i":
		return command.Inventory{}, nil

	case "equipment", "equip", "eq":
		return command.ShowEquipment{}, nil

	case "'", "say":
		return command.Say{Value: rest}, nil

	case "tell", "t":
		if len(tokens) < 3 {
			return nil, errors.New("usage: tell [somebody] [something]")
		}
		return command.Tell{
			To:    tokens[1],
			Value: strings.Join(tokens[2:], " "),
		}, nil

	case "tellall", "ta", "shout":
		return command.TellAll{Value: rest}, nil

	case "who":
		return command.Who{}, nil

	case "color", "colour":
		return command.Color{Setting: strings.ToLower(rest)}, nil

	case "stat", "stats":
		return command.Stat{}, nil

	case "role", "roles":
		return command.Role{}, nil

	case "cast", "c":
		// the first word is the ability; the rest is the target, raw
		if len(tokens) < 2 {
			return command.Cast{}, nil
		}
		return command.Cast{Ability: tokens[1], Target: strings.Join(tokens[2:], " ")}, nil

	case "consider", "con":
		return command.Consider{Target: rest}, nil

	case "kill", "attack":
		if len(tokens) < 2 {
			return nil, errors.New("What do you want to attack?")
		}
		return command.Kill{Target: tokens[1]}, nil

	case "flee":
		return command.Flee{}, nil

	case "restore":
		if len(tokens) < 2 {
			return nil, errors.New("Restore whom?")
		}
		return command.Restore{Target: tokens[1]}, nil

	case "roomstatus":
		return command.RoomStatus{}, nil

	case "load": // load (mob|obj) [zone] id
		switch len(tokens) {
		case 3:
			return command.Load{Type: tokens[1], Id: tokens[2]}, nil
		case 4:
			return command.Load{Type: tokens[1], Zone: tokens[2], Id: tokens[3]}, nil
		default:
			return nil, errors.New("try: `load (mob|obj) [zone] id`")
		}
	}
	return nil, errors.New("Unknown request: " + tokens[0])
}
