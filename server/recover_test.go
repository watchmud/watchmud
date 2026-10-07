package server

import (
	"errors"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/player"
)

func boom(*gameserver.HandlerParameter) error { panic("boom") }

// A handler that panics costs the player that one command, not everybody
// their session.
func TestRecovering_aPlayersCommand(t *testing.T) {
	gs, _ := newTestGameServer(t)
	r := &player.Recorder{}
	c := &testConn{p: player.NewTestPlayer(uuid.New(), "Bob", r)}

	assert.NotPanics(t, func() {
		gs.recovering(gameserver.NewHandlerParameter(c, command.Look{}), boom)
	})
	assert.Equal(t, []any{event.Failed{Verb: "look", Code: event.Unknown}}, r.Sent)
}

// Mid-login there is no player to tell, and the login conversation is
// blocked waiting for an answer that would never come: it gets a failure,
// which ends it, rather than hanging until the idle timeout.
func TestRecovering_aLogin(t *testing.T) {
	gs, _ := newTestGameServer(t)
	c := &testConn{}

	assert.NotPanics(t, func() {
		gs.recovering(gameserver.NewHandlerParameter(c, command.Login{Name: "Bob"}), boom)
	})
	assert.Equal(t, []any{event.LoginFailed{Reason: event.Unknown}}, c.sent)
}

// One pulse job panicking doesn't skip the ones after it -- the save comes
// last, and a crash in combat must not stop everyone being saved.
func TestRecoverPulse_theRestStillRun(t *testing.T) {
	ran := false
	assert.NotPanics(t, func() {
		recoverPulse("violence", func() { panic("boom") })
		recoverPulse("save", func() { ran = true })
	})
	assert.True(t, ran)
}

func storeDown(*gameserver.HandlerParameter) error { return errors.New("store unreachable") }

// And the same for a handler that returns an error -- the store having a bad
// moment -- which used to leave the conversation waiting for good, holding
// its address's slot.
func TestRecovering_aLoginThatErrs(t *testing.T) {
	gs, _ := newTestGameServer(t)
	c := &testConn{}
	gs.recovering(gameserver.NewHandlerParameter(c, command.Login{Name: "Bob"}), storeDown)
	assert.Equal(t, []any{event.LoginFailed{Reason: event.Unknown}}, c.sent)

	// a player's command that errs is logged; they're not told it was a login
	r := &player.Recorder{}
	p := &testConn{p: player.NewTestPlayer(uuid.New(), "Bob", r)}
	gs.recovering(gameserver.NewHandlerParameter(p, command.Look{}), storeDown)
	assert.Empty(t, p.sent)
}

// A login's last step panicking after the player was attached still answers
// the conversation, and takes the half-made login back out.
func TestRecovering_aLoginsLastStep(t *testing.T) {
	gs, _ := newTestGameServer(t)
	c := &testConn{}
	p := player.NewTestPlayer(uuid.New(), "Bob", nil)
	attachThenBoom := func(msg *gameserver.HandlerParameter) error {
		msg.Client.SetPlayer(p)
		gs.world.PlacePlayer(p, gs.world.StartRoom)
		panic("boom")
	}

	gs.recovering(gameserver.NewHandlerParameter(c, loginChecked{Name: "Bob", Ok: true}), attachThenBoom)

	assert.Equal(t, []any{event.LoginFailed{Reason: event.Unknown}}, c.sent)
	assert.Nil(t, c.Player())
	assert.False(t, gs.world.IsPlaying("Bob"))
}
