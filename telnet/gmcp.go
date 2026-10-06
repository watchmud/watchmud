package telnet

import (
	"bytes"
	"encoding/json"
	"hash/fnv"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/event"
)

// GMCP (Generic MUD Communication Protocol, telnet option 201) is a side
// channel of JSON a MUD client reads instead of the text: Mudlet hands it to
// scripts as the gmcp table, which is what health bars and auto-mappers are
// built from. The server offers it after the banner; a client that says DO
// gets, from then on:
//
//   - Char.Vitals {"hp", "maxhp", "mp", "maxmp"}, whenever they change
//   - Room.Info {"num", "id", "name", "area", "exits"}, on every room
//
// Nothing the world sends changes for it. Both come out of events the
// connection already sees -- event.Prompt, event.RoomDescription -- so GMCP
// is a second rendering of them, written raw beside the text. What a client
// sends back (Core.Hello, Core.Supports.Set) is ignored: there is one set of
// packages, and everyone who says DO gets it.
const optGMCP = 201

// gmcpOn is the client's answer to WILL GMCP, from readPump to writePump
// through the queue, like endOfRecord.
type gmcpOn bool

// vitals is what Char.Vitals reports, and what the connection remembers so it
// only sends a change.
type vitals struct {
	HP    int `json:"hp"`
	MaxHP int `json:"maxhp"`
	MP    int `json:"mp"`
	MaxMP int `json:"maxmp"`
}

// roomInfo is Room.Info, in the shape IRE's games made the convention:
// mapper scripts written for those read this one. num is what a mapper keys
// rooms on and must be a number; id is the same room by name.
type roomInfo struct {
	Num   uint32            `json:"num"`
	Id    string            `json:"id"`
	Name  string            `json:"name"`
	Area  string            `json:"area"`
	Exits map[string]uint32 `json:"exits"`
}

// roomNum is a room's number for a mapper: a hash of "zone/room", so it is
// the same on every server start and needs nothing stored. Collisions are a
// one-in-millions chance at this many rooms; a mapper that met one would
// merge two rooms on its map, not break the game.
func roomNum(ref string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(ref))
	return h.Sum32() & 0x7fffffff
}

// gmcpFor is what GMCP says about msg, if anything, given the vitals last
// sent; it returns the vitals to remember.
func gmcpFor(msg any, last *vitals) (string, *vitals) {
	switch m := msg.(type) {
	case event.Prompt:
		v := vitals{HP: m.CurrentHealth, MaxHP: m.MaxHealth, MP: m.CurrentMana, MaxMP: m.MaxMana}
		if last != nil && *last == v {
			return "", last
		}
		return gmcpMessage("Char.Vitals", v), &v
	case event.RoomDescription:
		info := roomInfo{Num: roomNum(m.Id), Id: m.Id, Name: m.Name, Area: m.Area, Exits: map[string]uint32{}}
		for _, ex := range m.ExitTo {
			info.Exits[strings.ToLower(ex.Direction.Abbrev())] = roomNum(ex.To)
		}
		return gmcpMessage("Room.Info", info), last
	}
	return "", last
}

// gmcpMessage frames one GMCP message: IAC SB GMCP, the package, a space and
// the JSON, IAC SE. A 0xFF in the payload is doubled, as any subnegotiation's
// must be -- UTF-8 never makes one, but the framing shouldn't depend on that.
func gmcpMessage(pkg string, data any) string {
	body, err := json.Marshal(data)
	if err != nil {
		log.Error().Err(err).Str("package", pkg).Msg("gmcp: can't encode")
		return ""
	}
	payload := bytes.ReplaceAll(append([]byte(pkg+" "), body...), []byte{IAC}, []byte{IAC, IAC})
	return string([]byte{IAC, SB, optGMCP}) + string(payload) + string([]byte{IAC, SE})
}
