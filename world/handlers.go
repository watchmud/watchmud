package world

import (
	"fmt"
	"github.com/watchmud/watchmud/player"

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
	if p := msg.Player; p != nil {
		// a frozen player can look around and leave, nothing else
		if _, ok := msg.Command.(command.Look); !ok && p.Frozen() {
			if _, quitting := msg.Command.(command.Logout); !quitting {
				msg.Fail(event.Frozen)
				return nil
			}
		}
		if _, talk := msg.Command.(command.Talk); talk && p.Muted() {
			msg.Fail(event.Muted)
			return nil
		}
		if p.Position() == player.Sleeping && !awake(msg.Command) {
			msg.Fail(event.Asleep)
			return nil
		}
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
	case command.Put:
		w.handlePut(msg, cmd)
	case command.Give:
		w.handleGive(msg, cmd)
	case command.Junk:
		w.handleJunk(msg, cmd)
	case command.Donate:
		w.handleDonate(msg, cmd)
	case command.Follow:
		w.handleFollow(msg, cmd)
	case command.Ungroup:
		w.handleUngroup(msg, cmd)
	case command.Group:
		w.handleGroup(msg, cmd)
	case command.Assist:
		w.handleAssist(msg, cmd)
	case command.OOC:
		w.handleOOC(msg, cmd)
	case command.Social:
		w.handleSocial(msg, cmd)
	case command.Socials:
		w.handleSocials(msg, cmd)
	case command.Emote:
		w.handleEmote(msg, cmd)
	case command.Reply:
		w.handleReply(msg, cmd)
	case command.Whisper:
		w.handleWhisper(msg, cmd)
	case command.Toggle:
		w.handleToggle(msg, cmd)
	case command.Goto:
		w.handleGoto(msg, cmd)
	case command.Transfer:
		w.handleTransfer(msg, cmd)
	case command.Purge:
		w.handlePurge(msg, cmd)
	case command.ZReset:
		w.handleZReset(msg, cmd)
	case command.Echo:
		w.handleEcho(msg, cmd)
	case command.Users:
		w.handleUsers(msg, cmd)
	case command.Moderate:
		w.handleModerate(msg, cmd)
	case command.Report:
		w.handleReport(msg, cmd)
	case command.Position:
		w.handlePosition(msg, cmd)
	case command.Split:
		w.handleSplit(msg, cmd)
	case command.Where:
		w.handleWhere(msg, cmd)
	case command.Commands:
		w.handleCommands(msg, cmd)
	case command.Time:
		w.handleTime(msg, cmd)
	case command.Wimpy:
		w.handleWimpy(msg, cmd)
	case command.Track:
		w.handleTrack(msg, cmd)
	case command.Reports:
		w.handleReports(msg, cmd)
	case command.GroupTell:
		w.handleGroupTell(msg, cmd)
	case command.Open:
		w.handleDoor(msg, cmd.Target, event.DoorOpened)
	case command.Close:
		w.handleDoor(msg, cmd.Target, event.DoorClosed)
	case command.Lock:
		w.handleDoor(msg, cmd.Target, event.DoorLocked)
	case command.Unlock:
		w.handleDoor(msg, cmd.Target, event.DoorUnlocked)
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
	case command.Gold:
		w.handleGold(msg, cmd)
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
