package telnet

import (
	"bytes"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

const (
	echoOff = "\xff\xfb\x01"     // IAC WILL ECHO
	echoOn  = "\xff\xfc\x01\r\n" // IAC WONT ECHO, and the newline their Enter didn't echo
	ga      = "\xff\xf9"         // IAC GA, after every prompt
	eor     = "\xff\xef"         // IAC EOR, instead, once the client says DO EOR
)

// fakeServer answers logins and creations the way GameServer does, from a
// map of names to passwords, and remembers every command it was sent.
type fakeServer struct {
	mu        sync.Mutex
	passwords map[string]string
	received  []command.Command
	loggedOut bool
	cause     string
}

func (f *fakeServer) Receive(msg *gameserver.HandlerParameter) {
	f.mu.Lock()
	f.received = append(f.received, msg.Command)
	f.mu.Unlock()

	switch cmd := msg.Command.(type) {
	case command.Login:
		want, exists := f.passwords[cmd.Name]
		switch {
		case len(cmd.Name) < 3:
			msg.Client.Send(event.LoginFailed{Reason: event.InvalidName})
		case cmd.Name == "rabbit":
			msg.Client.Send(event.LoginFailed{Reason: event.NameReserved})
		case !exists:
			msg.Client.Send(event.LoginFailed{Reason: event.NoSuchPlayer})
		case cmd.Password == "":
			msg.Client.Send(event.LoginFailed{Reason: event.PasswordRequired})
		case string(cmd.Password) != want:
			msg.Client.Send(event.LoginFailed{Reason: event.BadPassword})
		default:
			msg.Client.Send(event.LoggedIn{Name: cmd.Name})
		}
	case command.CreatePlayer:
		msg.Client.Send(event.PlayerCreated{Name: cmd.Name})
	}
}

func (f *fakeServer) Logout(_ gameserver.Conn, cause string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loggedOut = true
	f.cause = cause
}

func (f *fakeServer) logoutCause() (bool, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loggedOut, f.cause
}

func (f *fakeServer) commands() []command.Command {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]command.Command(nil), f.received...)
}

// session is the player's end of a net.Pipe, with everything the server
// writes collected as it arrives: a pipe write blocks until it's read.
type session struct {
	t      *testing.T
	client net.Conn
	mu     sync.Mutex
	out    bytes.Buffer
	closed chan struct{} // the server hung up
}

func startSession(t *testing.T, gs gameserver.Instance) *session {
	t.Helper()
	return startSessionWith(t, gs, nil)
}

// startSessionWith lets a test adjust the conn -- its timeouts, say -- before
// its pumps start.
func startSessionWith(t *testing.T, gs gameserver.Instance, adjust func(*conn)) *session {
	t.Helper()
	serverEnd, client := net.Pipe()
	s := &session{t: t, client: client, closed: make(chan struct{})}
	go func() {
		defer close(s.closed)
		buf := make([]byte, 256)
		for {
			n, err := client.Read(buf)
			s.mu.Lock()
			s.out.Write(buf[:n])
			s.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()

	c := newConn(serverEnd, gs, nil) // no catalog: creation skips the lineage menu
	if adjust != nil {
		adjust(c)
	}
	go c.writePump()
	go c.readPump()
	t.Cleanup(func() { _ = client.Close() })
	return s
}

// waitFor blocks until the server has written text, since typing ahead of a
// prompt would read fine but make the transcript assertions meaningless.
func (s *session) waitFor(text string) {
	s.t.Helper()
	require.Eventually(s.t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return bytes.Contains(s.out.Bytes(), []byte(text))
	}, 2*time.Second, time.Millisecond, "never saw %q in %q", text, s.transcript())
}

func (s *session) answer(prompt, line string) {
	s.t.Helper()
	s.waitFor(prompt)
	_, err := io.WriteString(s.client, line+"\r\n")
	require.NoError(s.t, err)
}

func (s *session) transcript() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.out.String()
}

func TestLogin_passwordIsNotEchoed(t *testing.T) {
	gs := &fakeServer{passwords: map[string]string{"bob": "sekrit99"}}
	s := startSession(t, gs)

	s.answer("known? ", "bob")
	s.answer("Password: ", "sekrit99")
	s.waitFor(echoOn)

	assert.Contains(t, s.transcript(), echoOff+"Password: "+ga+echoOn)
	assert.Equal(t, []command.Command{
		command.Login{Name: "bob"},
		command.Login{Name: "bob", Password: "sekrit99"},
	}, gs.commands())
}

func TestLogin_wrongPasswordAsksAgain(t *testing.T) {
	gs := &fakeServer{passwords: map[string]string{"bob": "sekrit99"}}
	s := startSession(t, gs)

	s.answer("known? ", "bob")
	s.answer("Password: ", "guess")
	s.answer("Wrong password.\r\n"+echoOff+"Password: ", "sekrit99")
	require.Eventually(t, func() bool { return len(gs.commands()) == 3 }, 2*time.Second, time.Millisecond)

	cmds := gs.commands()
	assert.Equal(t, command.Login{Name: "bob", Password: "sekrit99"}, cmds[2])
	assert.NotContains(t, s.transcript(), "Create them?", "a wrong password isn't an unknown name")
}

func TestLogin_threeWrongPasswordsHangsUp(t *testing.T) {
	gs := &fakeServer{passwords: map[string]string{"bob": "sekrit99"}}
	s := startSession(t, gs)

	s.answer("known? ", "bob")
	for range passwordTries {
		s.answer("Password: ", "guess")
		s.waitFor("Wrong password.")
	}

	select {
	case <-s.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("still connected after three wrong passwords")
	}
	assert.Contains(t, s.transcript(), "Too many wrong passwords.")
	assert.Len(t, gs.commands(), 1+passwordTries)
}

func TestCreate_passwordIsConfirmed(t *testing.T) {
	gs := &fakeServer{passwords: map[string]string{}}
	s := startSession(t, gs)

	s.answer("known? ", "newbie")
	s.answer("Create them? (yn) ", "y")
	s.answer("Choose a password: ", "short")
	s.answer("at least 8", "longenough")
	s.answer("Again: ", "different1")
	s.answer("don't match", "longenough")
	s.answer("Again: ", "longenough")
	s.waitFor(echoOn)

	cmds := gs.commands()
	require.Len(t, cmds, 2)
	assert.Equal(t, command.CreatePlayer{Name: "Newbie", Password: "longenough"}, cmds[1])

	// every one of those five answers was typed with echo off
	transcript := s.transcript()
	for _, p := range []string{"Choose a password: ", "Again: "} {
		assert.Contains(t, transcript, echoOff+p)
	}
}

func TestCheckPassword(t *testing.T) {
	assert.Equal(t, "", checkPassword("eightchr"))
	assert.Equal(t, "", checkPassword(string(bytes.Repeat([]byte("x"), 72))))
	assert.Contains(t, checkPassword("seven77"), "at least 8")
	assert.Contains(t, checkPassword(string(bytes.Repeat([]byte("x"), 73))), "72")
	// eight characters, but the limit is bcrypt's, in bytes: 8 × 3-byte runes
	assert.Equal(t, "", checkPassword("日本語日本語日本"))
	assert.Contains(t, checkPassword(string(bytes.Repeat([]byte("é"), 37))), "72")
}

// A name the server won't have is explained and asked for again, rather than
// hanging up or offering to create it.
func TestLogin_badNamesAskAgain(t *testing.T) {
	gs := &fakeServer{passwords: map[string]string{}}
	s := startSession(t, gs)

	s.answer("known? ", "xy")
	s.answer("letters", "rabbit")
	s.answer("belongs to", "newbie")
	s.waitFor("Create them?")

	transcript := s.transcript()
	assert.Contains(t, transcript, "Names are 3 to 16 letters, a to z, and nothing else.")
	assert.Contains(t, transcript, "That name belongs to something else here. Pick another.")
	assert.Contains(t, transcript, "No one by the name of Newbie.", "shown as it will be stored")
}
