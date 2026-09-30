package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/spaces"
)

func (w *World) handleRoomStatus(msg *gameserver.HandlerParameter, cmd command.RoomStatus) {
	// TODO allow user to specify room and zone to get status of

	room := w.playerRoom(msg.Player)

	msg.Player.Send(event.RoomStatus{
		Id:          room.Id,
		Name:        room.Name,
		Description: room.Description,
		ZoneName:    room.Zone.Name,
		ZoneId:      room.Zone.Id,
		Flags:       room.Flags(),
		Players:     createPlayerInfo(room),
		Items:       createInventoryInfo(room),
		Mobs:        createMobInfo(room),
		Exits:       createDirections(room),
	})
}

func createPlayerInfo(room *spaces.Room) (result []event.RoomStatusPlayer) {
	for _, p := range room.Players() {
		result = append(result, event.RoomStatusPlayer{
			Name:          p.Name(),
			CurrentHealth: 0, // TODO
			MaxHealth:     0, // TODO
		})
	}
	return
}

func createInventoryInfo(room *spaces.Room) (result []event.RoomStatusItem) {
	for i := range room.Inventory.All() {
		result = append(result,
			event.RoomStatusItem{
				Id:                  i.Id.String(),
				DefinitionId:        i.Definition.ObjectId.DefinitionId,
				Aliases:             i.Definition.Aliases,
				Category:            i.Definition.ObjectCategory,
				Name:                i.Definition.Name,
				ShortDescription:    i.Definition.ShortDescription,
				DescriptionOnGround: i.Definition.DescriptionOnGround,
				ZoneId:              i.Definition.ObjectId.ZoneId,
				Behaviors:           i.Definition.Behaviors,
			})
	}
	return
}

func createMobInfo(room *spaces.Room) (result []event.RoomStatusMob) {
	for _, m := range room.Mobiles() {
		result = append(result,
			event.RoomStatusMob{
				Id:                m.IdStr(),
				DefinitionId:      m.Definition.Id,
				Aliases:           m.Definition.Aliases,
				Name:              m.Definition.Name,
				ShortDescription:  m.Definition.ShortDescription,
				DescriptionInRoom: m.Definition.DescriptionInRoom,
				ZoneId:            m.Definition.ZoneId,
				CurrentHealth:     m.CurHealth,
				MaxHealth:         m.Definition.MaxHealth,
				Flags:             m.Definition.GetFlags(),
			})
	}
	return
}

func createDirections(room *spaces.Room) (result []event.RoomStatusExit) {
	for _, ex := range room.Exits(false) {
		result = append(result,
			event.RoomStatusExit{
				Direction: ex.Direction,
				RoomId:    ex.Room.Id,
				ZoneId:    ex.Room.Zone.Id,
				Flags:     ex.Room.Flags(),
			})
	}
	return
}
