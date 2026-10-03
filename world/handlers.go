package world

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

// HandleIncomingMessage runs the handler for one command.
//
// The switch is the whole dispatch table: no string keys, no reflection, and
// each handler is handed its command already typed.
//
// Builder commands are refused here, before the switch, so no handler has to
// remember to check. A player who isn't a wizard gets the answer a verb
// nobody has heard of gets.
func (w *World) HandleIncomingMessage(msg *gameserver.HandlerParameter) error {
	if _, wiz := msg.Command.(command.Wizard); wiz && (msg.Player == nil || !msg.Player.IsWizard()) {
		if msg.Player != nil {
			logWizCommand(msg.Player, msg.Command.Verb(), "Player %s is not a wizard; refused", msg.Player.Name())
		}
		msg.Fail(event.UnknownCommand)
		return nil
	}
	switch cmd := msg.Command.(type) {
	case command.Abilities:
		w.handleAbilities(msg, cmd)
	case command.Drop:
		w.handleDrop(msg, cmd)
	case command.Equip:
		w.handleEquip(msg, cmd)
	case command.Exits:
		w.handleExits(msg, cmd)
	case command.Flee:
		w.handleFlee(msg, cmd)
	case command.Get:
		w.handleGet(msg, cmd)
	case command.Inventory:
		w.handleInventory(msg, cmd)
	case command.Kill:
		w.handleKill(msg, cmd)
	case command.Consider:
		w.handleConsider(msg, cmd)
	case command.Load:
		w.handleLoad(msg, cmd)
	case command.Logout:
		w.handleLogout(msg, cmd)
	case command.Look:
		w.handleLook(msg, cmd)
	case command.Move:
		w.handleMove(msg, cmd)
	case command.Color:
		w.handleColor(msg, cmd)
	case command.NoHassle:
		w.handleNoHassle(msg, cmd)
	case command.Ping:
		w.handlePing(msg, cmd)
	case command.Recall:
		w.handleRecall(msg, cmd)
	case command.Repair:
		w.handleRepair(msg, cmd)
	case command.List:
		w.handleList(msg, cmd)
	case command.Buy:
		w.handleBuy(msg, cmd)
	case command.Sell:
		w.handleSell(msg, cmd)
	case command.Value:
		w.handleValue(msg, cmd)
	case command.Remove:
		w.handleRemove(msg, cmd)
	case command.Restore:
		w.handleRestore(msg, cmd)
	case command.Role:
		w.handleRole(msg, cmd)
	case command.Cast:
		w.handleCast(msg, cmd)
	case command.RoomStatus:
		w.handleRoomStatus(msg, cmd)
	case command.Slay:
		w.handleSlay(msg, cmd)
	case command.Say:
		w.handleSay(msg, cmd)
	case command.ShowEquipment:
		w.handleShowEquipment(msg, cmd)
	case command.Stat:
		w.handleStat(msg, cmd)
	case command.Tell:
		w.handleTell(msg, cmd)
	case command.TellAll:
		w.handleTellAll(msg, cmd)
	case command.Wear:
		w.handleWear(msg, cmd)
	case command.Who:
		w.handleWho(msg, cmd)
	default:
		log.Warn().Msgf("world.HandleIncomingMessage: UNHANDLED command %T", msg.Command)
		msg.Fail(event.UnknownCommand)
		return fmt.Errorf("unhandled command %T", msg.Command)
	}
	return nil
}
