package spaces

import (
	"github.com/watchmud/watchmud/rules"
)

// RoomExit is a direction to another room, and the door in it if it has one.
type RoomExit struct {
	Direction rules.Direction
	Room      *Room
	Door      *Door
}

// Passable is whether something can walk through it now: no door, or an open one.
func (re RoomExit) Passable() bool {
	return re.Door == nil || !re.Door.Closed
}

type roomExitHolder struct {
	dirs []RoomExit
}

func (re roomExitHolder) Len() int           { return len(re.dirs) }
func (re roomExitHolder) Less(i, j int) bool { return re.dirs[i].Direction < re.dirs[j].Direction }
func (re roomExitHolder) Swap(i, j int)      { re.dirs[i], re.dirs[j] = re.dirs[j], re.dirs[i] }
