package telnet

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/rules"
)

const (
	gmcpStart = "\xff\xfa\xc9" // IAC SB GMCP
	gmcpEnd   = "\xff\xf0"     // IAC SE
)

func TestGMCPMessage_framing(t *testing.T) {
	got := gmcpMessage("Char.Vitals", vitals{HP: 97, MaxHP: 100, MP: 80, MaxMP: 100})
	assert.Equal(t, gmcpStart+`Char.Vitals {"hp":97,"maxhp":100,"mp":80,"maxmp":100}`+gmcpEnd, got)
}

// gmcpPayloads is every GMCP message in out, as "Package json".
func gmcpPayloads(out string) []string {
	var found []string
	for {
		i := strings.Index(out, gmcpStart)
		if i < 0 {
			return found
		}
		out = out[i+len(gmcpStart):]
		j := strings.Index(out, gmcpEnd)
		found = append(found, out[:j])
		out = out[j+len(gmcpEnd):]
	}
}

var wounded = event.Prompt{CurrentHealth: 97, MaxHealth: 100, CurrentMana: 80, MaxMana: 100}

// A client that never said DO gets text, and nothing else.
func TestGMCP_offUntilTheClientSaysDo(t *testing.T) {
	c := loggedInConn()

	assert.Empty(t, gmcpPayloads(writes(t, c, wounded)))
}

// Vitals go ahead of the prompt, and again only when they change -- the
// server sends a prompt every second, and a client wants news, not noise.
func TestGMCP_vitalsWhenTheyChange(t *testing.T) {
	c := loggedInConn()

	out := writes(t, c, gmcpOn(true), wounded, inputReceived{}, wounded)
	assert.Equal(t, []string{`Char.Vitals {"hp":97,"maxhp":100,"mp":80,"maxmp":100}`}, gmcpPayloads(out))
	assert.True(t, strings.HasPrefix(out, gmcpStart), "ahead of the prompt it goes with")

	healed := wounded
	healed.CurrentHealth = 100
	assert.Equal(t, []string{`Char.Vitals {"hp":100,"maxhp":100,"mp":80,"maxmp":100}`},
		gmcpPayloads(writes(t, c, healed)))
}

func TestGMCP_roomInfo(t *testing.T) {
	c := loggedInConn()
	room := event.RoomDescription{
		Name:  "Temple Square",
		Exits: "North, South",
		Id:    "wrathrock/temple_square",
		Area:  "Wrathrock",
		ExitTo: []event.ExitTo{
			{Direction: rules.DirectionNorth, To: "sample/start"},
			{Direction: rules.DirectionSouth, To: "wrathrock/market_square"},
		},
		X: 2, Y: -1,
	}

	out := writes(t, c, gmcpOn(true), room)
	payloads := gmcpPayloads(out)
	require.Len(t, payloads, 1)
	pkg, body, _ := strings.Cut(payloads[0], " ")
	assert.Equal(t, "Room.Info", pkg)

	var info roomInfo
	require.NoError(t, json.Unmarshal([]byte(body), &info))
	assert.Equal(t, roomInfo{
		Num:  roomNum("wrathrock/temple_square"),
		Id:   "wrathrock/temple_square",
		Name: "Temple Square",
		Area: "Wrathrock",
		Exits: map[string]uint32{
			"n": roomNum("sample/start"),
			"s": roomNum("wrathrock/market_square"),
		},
		Grid: grid{X: 2, Y: -1},
	}, info)
	assert.Contains(t, out, "Temple Square\r\n", "and the room as text, as ever")
}

// A room's number is the same every time, and different rooms differ.
func TestRoomNum(t *testing.T) {
	assert.Equal(t, roomNum("wrathrock/temple_square"), roomNum("wrathrock/temple_square"))
	assert.NotEqual(t, roomNum("wrathrock/temple_square"), roomNum("wrathrock/market_square"))
	assert.LessOrEqual(t, roomNum("wrathrock/temple_square"), uint32(0x7fffffff), "fits a signed int, for Lua")
}

func TestGMCP_clientSaysDoAndDont(t *testing.T) {
	c := loggedInConn()
	c.sendQueue = make(chan any, 2)

	c.negotiated(DO, optGMCP)
	c.negotiated(DONT, optGMCP)

	assert.Equal(t, gmcpOn(true), <-c.sendQueue)
	assert.Equal(t, gmcpOn(false), <-c.sendQueue)
}

// Two rooms with one number would be one room on a player's map. A hash can
// collide, so check the real world's rooms, every one.
func TestRoomNum_uniqueInTheWorld(t *testing.T) {
	content, err := loader.LoadContent(os.DirFS("../content"))
	require.NoError(t, err)
	seen := map[uint32]string{}
	for zid, z := range content.Zones {
		for rid := range z.Rooms {
			ref := zid + "/" + rid
			if other, dup := seen[roomNum(ref)]; dup {
				t.Errorf("%s and %s have the same map number", ref, other)
			}
			seen[roomNum(ref)] = ref
		}
	}
	assert.Greater(t, len(seen), 20)
}

// Lines said to the player go out as Comm.Channel.Text too, for a client's
// chat capture -- but not your own tell's "Ok.".
func TestGMCP_channelText(t *testing.T) {
	c := loggedInConn() // testdood
	out := writes(t, c, gmcpOn(true),
		event.Said{Speaker: "Ann", Value: "hello"},
		event.Told{From: "Ann", To: "testdood", Value: "psst"},
		event.Told{From: "testdood", To: "Ann", Value: "hi"},
		event.OOCSaid{Speaker: "Bob", Value: "anyone?"},
		event.GroupTold{Speaker: "testdood", Value: "go"},
		event.Pong{Target: "testdood"})
	assert.Equal(t, []string{
		`Comm.Channel.Text {"channel":"say","talker":"Ann","text":"Ann says, \"hello\"."}`,
		`Comm.Channel.Text {"channel":"tell","talker":"Ann","text":"Ann tells you, \"psst\"."}`,
		`Comm.Channel.Text {"channel":"ooc","talker":"Bob","text":"[ooc] Bob: anyone?"}`,
		`Comm.Channel.Text {"channel":"gtell","talker":"testdood","text":"You tell the group, 'go'"}`,
	}, gmcpPayloads(out))
}
