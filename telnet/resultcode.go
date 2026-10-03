package telnet

import (
	"strings"

	"github.com/rs/zerolog/log"
)

// failureText turns a handler's ResultCode into something a player can read.
//
// This table is keyed on the strings world/ emits, not on protobuf types, so
// unlike render.go it survives the Phase 5 decision unchanged.
//
// verb is the command the player typed. Most codes mean the same thing
// everywhere; the few that don't get a "verb/CODE" entry that wins.
func failureText(verb, code string) string {
	if code == "" {
		return "You can't do that.\n"
	}
	// the same words parse.go uses for a verb it doesn't know, so a builder
	// command refused to a player reads as one that doesn't exist
	if code == "UNKNOWN_COMMAND" {
		return "Unknown request: " + verb + "\n"
	}
	if s, ok := failureByVerb[verb+"/"+code]; ok {
		return s + "\n"
	}
	if s, ok := failureByCode[code]; ok {
		return s + "\n"
	}
	// TODO fix later ...
	// h_drop.go:20, h_equip.go:30 and h_kill.go:49 build codes by concatenating
	// an error onto a prefix, so the suffix is arbitrary — sometimes a raw
	// strconv message. Never show it to the player.
	for prefix, s := range failureByPrefix {
		if strings.HasPrefix(code, prefix) {
			return s + "\n"
		}
	}
	log.Warn().Msgf("telnet: no failure text for %q (verb %q)", code, verb)
	return "You can't do that.\n"
}

var failureByVerb = map[string]string{
	"color/BAD_REQUEST":       "Color on, color off, or just color to switch it.",
	"nohassle/BAD_REQUEST":    "Nohassle on, nohassle off, or just nohassle to switch it.",
	"drop/TARGET_NOT_FOUND":   "You aren't carrying that.",
	"repair/TARGET_NOT_FOUND": "You aren't carrying that.",
	"sell/TARGET_NOT_FOUND":   "You aren't carrying that.",
	"value/TARGET_NOT_FOUND":  "You aren't carrying that.",
	"sell/TARGET_IN_USE":      "You'll have to take that off first.",
	"wear/TARGET_NOT_FOUND":   "You aren't carrying that.",
	"equip/TARGET_NOT_FOUND":  "You aren't carrying that.",
	// remove searches the slots, not the inventory: not finding it means it
	// isn't equipped, which is a different sentence from not carrying it.
	"remove/TARGET_NOT_FOUND": "You aren't using that.",
	"remove/NO_TARGET":        "Remove what?",
	"slay/NO_TARGET":          "Slay what?",
	"consider/NO_TARGET":      "Consider what?",
	"look/NO_TARGET":          "Look in what?",
	"drop/NO_TARGET":          "Drop what?",
	"get/NO_TARGET":           "Get what?",
	"wear/NO_TARGET":          "Wear what?",
	"equip/NO_TARGET":         "Wield what?",
	"equip/NO_SLOT_GIVEN":     "Wield it where?",
	"cast/NO_TARGET":          "Cast what?",
	"cast/TARGET_NOT_FOUND":   "There's no one here by that name.",
}

var failureByCode = map[string]string{
	// the parser and the dispatcher, not a handler
	"PARSE_ERROR":     "You'll have to phrase that differently.",
	"UNKNOWN_COMMAND": "I don't understand that.",
	"INTERNAL_ERROR":  "Something went wrong.",
	"UNKNOWN":         "Something went wrong there. It's been logged.", // a handler panicked

	// targets
	"TARGET_NOT_FOUND":    "You don't see that here.",
	"TARGET_NOT_GETTABLE": "You can't pick that up.",
	"NO_TARGET":           "You'll have to be more specific.",
	"NO_SMITH":            "There's no one here who can repair that. Try the smithy in Wrathrock.",
	"NOT_DAMAGED":         "That doesn't need repairing.",
	"NO_SHOP":             "There's no one here to trade with. Try the General Store in Wrathrock.",
	"NOT_FOR_SALE":        "That isn't for sale here. Type 'list' to see what is.",
	"WORTHLESS":           "The shopkeeper isn't interested in that.",

	// abilities
	"UNKNOWN_ABILITY": "There's no such spell.",
	"NOT_GRANTED":     "Nothing you're wearing lets you cast that.",
	"NOT_READY":       "You can't cast that again yet.",
	"NOT_ENOUGH_MANA": "You don't have enough mana.",

	// containers
	"NOT_A_CONTAINER":  "That's not a container.",
	"NOT_IN_CONTAINER": "You don't see that in there.",
	"CONTAINER_EMPTY":  "There's nothing in there.",

	// carrying and wearing
	"TARGET_IN_USE":   "You'll have to remove it first.",
	"IN_USE":          "You're already wearing that.",
	"LOCATION_IN_USE": "You're already using that slot.",
	"CANT_WEAR_THAT":  "You can't wear that.",
	"CANT_WEAR_THERE": "You can't wear that there.",

	// movement and combat
	"CANT_GO_THAT_WAY": "You can't go that way.",
	"NO_FIGHT_ROOM":    "You feel far too peaceful to fight here.",
	"NO_FIGHT":         "You can't attack that.",
	"IN_A_FIGHT":       "You're too busy fighting!",
	"ALREADY_FIGHTING": "You're already fighting!",

	// talking
	"TO_PLAYER_NOT_FOUND": "No one by that name is playing.",
	"NO_VALUE":            "Say what?",

	// three spellings of one broken state (h_roomstatus.go:19, h_say.go:16,
	// h_wiz_load.go:19). Don't go fix the handlers; Phase 5 may rewrite them.
	"NOT_IN_ROOM":           "You're nowhere at all.",
	"NOT_IN_A_ROOM":         "You're nowhere at all.",
	"YOU_ARE_NOT_IN_A_ROOM": "You're nowhere at all.",

	// builder commands (h_wiz_load.go) — deliberately technical; the audience
	// is someone editing content/
	"UNKNOWN_TYPE":          "Unknown type: try mob or obj.",
	"UNKNOWN_ZONE":          "No such zone.",
	"UNKNOWN_ID":            "No such id.",
	"UNKNOWN_DEFINITION_ID": "No such definition id.",

	// internal failures: the player did nothing wrong
	"ADD_TO_ROOM_ERROR":         "Something went wrong.",
	"REMOVE_FROM_ROOM_ERROR":    "Something went wrong.",
	"ADD_ROOM_INVENTORY_FAILED": "Something went wrong.",
	"DATA_ERROR":                "Something went wrong.",

	"UNKNOWN_MESSAGE_TYPE": "I don't understand that.",
}

var failureByPrefix = map[string]string{
	"PARSE_ERROR_": "You'll have to phrase that differently.",
	"ERROR_":       "Something went wrong.",
}
