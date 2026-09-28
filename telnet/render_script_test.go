package telnet

import (
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/world"
)

// hecklerHere brings testcontent's scripted heckler from the general store
// into the start room.
func hecklerHere(w *world.World, _ *player.Player, _ *player.Player) {
	heckler, found := w.Zone("wrathrock").Rooms["general_store"].FindMobile("heckler")
	if !found {
		panic("testcontent has no heckler")
	}
	w.RemoveMobile(heckler)
	w.PlaceMobile(heckler, w.StartRoom)
}

var scriptCases = []commandCase{
	{
		name:      "a scripted mob's opener",
		setup:     hecklerHere,
		input:     "kill heckler",
		want:      "Ok.\nHeckler says, \"Come on then, testdood!\".\n",
		wantOther: "Heckler says, \"Come on then, testdood!\".\n",
	},
}
