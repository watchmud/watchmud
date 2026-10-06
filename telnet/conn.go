package telnet

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

type conn struct {
	gs         gameserver.Instance
	cat        *rules.Catalog // read-only; the creation menu is built from it
	netConn    net.Conn
	scanner    *bufio.Scanner
	sendQueue  chan any // NOT *message.GameMessage - see below.
	quit       chan struct{}
	closeOnce  sync.Once
	authResult chan event.ResultCode // buffered 1; how login/create ended: "" is success, anything else is why not

	mu     sync.Mutex // guards the player
	player *player.Player

	// atPrompt is true while the last thing written was the prompt and the
	// player hasn't answered it. lastPrompt is the most recent one the server
	// sent, for repromptIn to repeat. Both owned by writePump; nothing else
	// touches them.
	atPrompt   bool
	lastPrompt *event.Prompt

	// eor is whether the client agreed to IAC EOR as the end of a prompt;
	// until it does, a prompt ends with IAC GA. Owned by writePump.
	eor bool
	// color is whether this player wants ANSI color, from the event.Color
	// the world sends at login and on the color command. Off until then, so
	// the login conversation is plain. Owned by writePump.
	color bool
	// width is the client's window, to wrap to; zero until it says, and then
	// no wrapping. Owned by writePump.
	width int
	// gmcp is whether the client said DO GMCP, and vitals the Char.Vitals
	// last sent, so only a change goes. Owned by writePump.
	gmcp   bool
	vitals *vitals

	// How long a line may take to arrive before the connection is dropped,
	// before login and after. readPump owns readTimeout and switches it
	// from one to the other.
	loginIdle, playIdle time.Duration
	readTimeout         time.Duration
	// loginBy is when the login conversation must be over, however lively:
	// answering "help" every minute isn't playing, it's holding a slot.
	loginBy time.Time

	// onClose runs once the socket is closed, to give back its slot.
	onClose func()
	// mayCreate and created are the address's allowance of new characters
	// (addressLimit); nil, as in tests, is no limit.
	mayCreate func() bool
	created   func()
}

const (
	// Short: someone sitting at the name prompt holds a connection slot and
	// is doing nothing else with it.
	defaultLoginIdle = 2 * time.Minute
	// Long: a player reading, or making tea. Dropping them ends the session
	// the way quitting does, saved and out of the world.
	defaultPlayIdle = 30 * time.Minute
)

// inputReceived goes through the send queue when readPump reads a line, so
// writePump knows the prompt it wrote has been answered: the client's echo
// took the cursor to a new line, and the next thing written needs a new
// prompt after it. Through the queue rather than a field so it lands in
// order, ahead of whatever the world says in reply.
type inputReceived struct{}

// echo turns the client's local echo on or off, so a password isn't shown
// as it's typed. Through the send queue so it lands in order around the
// prompt, and written raw by write: it is protocol, not text, and neither
// frame nor render has any business with it.
type echo bool

// question is a prompt of the login conversation: plain text, but a prompt,
// so it's marked as one the way the in-game prompt is.
type question string

// negotiation is telnet option bytes the server offers, written raw.
type negotiation string

// endOfRecord is the client's answer to the server's WILL EOR: true for DO,
// false for DONT. Through the send queue, since it's read on readPump and
// writePump is the one that marks prompts.
type endOfRecord bool

// reprompt asks writePump for the prompt again, for the input the world
// never hears about. The connection can't build one itself: what the prompt
// shows is world state, and this side of the seam doesn't read that. So it
// repeats the last one the server sent.
type reprompt struct{}

func newConn(c net.Conn, gs gameserver.Instance, cat *rules.Catalog) *conn {
	const sendQueueSize = 256
	return &conn{
		gs:         gs,
		cat:        cat,
		netConn:    c,
		sendQueue:  make(chan any, sendQueueSize),
		quit:       make(chan struct{}),
		authResult: make(chan event.ResultCode, 1),
		loginIdle:  defaultLoginIdle,
		playIdle:   defaultPlayIdle,
	}
}

// Listen takes the catalog as well as the game server because character
// creation is a conversation held on this side of the seam, before there is a
// player to hand a command to, and a menu of lineages is presentation. The
// renderer already depends on rules for the same reason.
// Options says where to listen. TLSAddr empty means no TLS port.
type Options struct {
	Addr string // plain telnet

	TLSAddr           string
	TLSPort           int // told to players on the plain port, so they can find it
	CertFile, KeyFile string
}

func Listen(ctx context.Context, opts Options, gs gameserver.Instance, cat *rules.Catalog) error {
	// one cap across both ports: TLS isn't five more connections
	limit := &addressLimit{max: maxConnsPerAddress, open: make(map[string]int)}
	listeners := []listener{}

	ln, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		return fmt.Errorf("telnet listen on %s: %w", opts.Addr, err)
	}
	log.Info().Msgf("telnet listening on %s", opts.Addr)
	tlsPort := 0
	if opts.TLSAddr != "" {
		tlsPort = opts.TLSPort
	}
	listeners = append(listeners, listener{ln: ln, banner: plainBanner(tlsPort)})

	if opts.TLSAddr != "" {
		cert, err := loadCertificate(opts.CertFile, opts.KeyFile)
		if err != nil {
			_ = ln.Close()
			return err
		}
		tln, err := net.Listen("tcp", opts.TLSAddr)
		if err != nil {
			_ = ln.Close()
			return fmt.Errorf("tls listen on %s: %w", opts.TLSAddr, err)
		}
		log.Info().Msgf("telnet over TLS listening on %s", opts.TLSAddr)
		listeners = append(listeners, listener{
			ln:               tln,
			tls:              tlsConfig(cert),
			handshakeTimeout: defaultHandshakeTimeout,
			banner:           "Welcome to WatchMUD.\r\n",
		})
	}

	// the first to fail takes the rest down with it: a server that has
	// quietly lost one of its ports looks fine until someone tries it
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errs := make(chan error, len(listeners))
	for _, l := range listeners {
		go func() { errs <- serve(ctx, l, gs, cat, limit) }()
	}
	err = <-errs
	cancel()
	for range len(listeners) - 1 {
		<-errs
	}
	return err
}

// Serve is plain telnet on a listener the caller opened, for a caller that
// picks its own port -- bot's test listens on 127.0.0.1:0 and needs to know
// where. It closes ln when ctx is done. The server itself starts with Listen.
func Serve(ctx context.Context, ln net.Listener, gs gameserver.Instance, cat *rules.Catalog) error {
	limit := &addressLimit{max: maxConnsPerAddress, open: make(map[string]int)}
	return serve(ctx, listener{ln: ln, banner: plainBanner(0)}, gs, cat, limit)
}

// plainBanner greets a telnet connection, and points it at the TLS port when
// there is one.
func plainBanner(tlsPort int) string {
	banner := "Welcome to WatchMUD.\r\n"
	if tlsPort != 0 {
		banner += fmt.Sprintf("For an encrypted connection, use port %d with TLS.\r\n", tlsPort)
	}
	return banner
}

// maxConnsPerAddress is enough for a household behind one router, or a player
// with a second window open, and not enough for one client to take every
// slot. Everyone behind a TLS-terminating proxy shares its address, so a
// proxy in front of this needs PROXY protocol, or this needs to go up.
const maxConnsPerAddress = 5

// loginWithin is how long a connection has to log in, start to finish.
const loginWithin = 5 * time.Minute

// maxConns is every connection together, logged in or not: each one is two
// goroutines and a socket, and an attacker with many addresses -- an IPv6
// /64 is a lot of them -- shouldn't be able to take them all.
const maxConns = 200

// listener is one port: plain telnet when tls is nil.
type listener struct {
	ln               net.Listener
	tls              *tls.Config
	handshakeTimeout time.Duration
	banner           string
}

// serve accepts connections from l until ctx is done, refusing any past the
// limit from one IP.
func serve(ctx context.Context, l listener, gs gameserver.Instance, cat *rules.Catalog, limit *addressLimit) error {
	go func() {
		<-ctx.Done()
		_ = l.ln.Close() // unblocks the Accept, below
	}()

	for {
		nc, err := l.ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil // shutting down, expected
			}
			return fmt.Errorf("telnet accept: %w", err)
		}
		host := remoteHost(nc)
		if ok, full := limit.acquire(host); !ok {
			log.Warn().Msgf("telnet %s: too many connections from %s (full: %t), refused", nc.RemoteAddr(), host, full)
			why := "Too many connections from your address. Try again later.\r\n"
			if full {
				why = "The game is full right now. Try again later.\r\n"
			}
			if l.tls == nil {
				refuse(nc, why)
			} else {
				_ = nc.Close() // nothing it could read yet: that takes a handshake
			}
			continue
		}
		if l.tls == nil {
			start(nc, gs, cat, l.banner, host, limit)
			continue
		}
		// the handshake on its own goroutine: a slow or silent client must
		// not hold up the next Accept
		go func() {
			tc := tls.Server(nc, l.tls)
			_ = tc.SetDeadline(time.Now().Add(l.handshakeTimeout))
			if err := tc.Handshake(); err != nil {
				log.Info().Msgf("telnet %s: TLS handshake failed: %v", nc.RemoteAddr(), err)
				_ = nc.Close()
				limit.release(host)
				return
			}
			_ = tc.SetDeadline(time.Time{}) // the conn's pumps set their own
			start(tc, gs, cat, l.banner, host, limit)
		}()
	}
}

// start runs a connection that has its slot, and gives the slot back when it
// closes.
func start(nc net.Conn, gs gameserver.Instance, cat *rules.Catalog, banner, host string, limit *addressLimit) {
	log.Info().Msgf("telnet connection from %s", nc.RemoteAddr())
	c := newConn(nc, gs, cat)
	c.onClose = func() { limit.release(host) }
	c.mayCreate = func() bool { return limit.mayCreate(host, time.Now()) }
	c.created = func() { limit.created(host, time.Now()) }
	go c.writePump()
	go c.readPump()
	c.Send(banner)
	// after the banner, which is what a person reads first: a MUD client
	// that says DO EOR gets EOR after each prompt instead of GA, and one that
	// says WILL NAWS tells us how wide to wrap, and one that says DO GMCP
	// gets vitals and rooms as data (gmcp.go)
	c.Send(negotiation([]byte{IAC, WILL, optEOR, IAC, DO, optNAWS, IAC, WILL, optGMCP}))
}

func (c *conn) Player() *player.Player {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.player
}

func (c *conn) SetPlayer(p *player.Player) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.player = p
}

// Send sends a message for the player.Sender interface
func (c *conn) Send(msg any) {
	_ = c.send(msg)
}

// Send handles sending messages to the telnet client.
// This can return an error if there's a legitimate sending error,
// which makes it distinctive from player.Sender.Send.
// Hooks into the login state signaling completion of login/create.
func (c *conn) send(msg any) error {
	// the login conversation, below, is waiting on these four.
	switch m := msg.(type) {
	case event.LoggedIn, event.PlayerCreated:
		c.signalAuth("")
		return nil
	case event.LoginFailed:
		c.signalAuth(m.Reason)
		return nil
	case event.CreateFailed:
		c.signalAuth(m.Reason)
		return nil
	}
	select {
	case <-c.quit:
		// already hung up: the room's chatter has nowhere to go, and saying
		// so once was enough
		return errors.New("connection closed")
	default:
	}
	select {
	case c.sendQueue <- msg:
		return nil
	default:
		log.Warn().Msgf("telnet %s: send queue full, closing", c.netConn.RemoteAddr())
		c.Close()
		return errors.New("send queue full")
	}
}

func (c *conn) signalAuth(why event.ResultCode) {
	select {
	case c.authResult <- why:
	default: // don't block if the authResult channel is full
	}
}

// awaitAuth waits for the server's answer to a login or a creation: whether
// it worked, and if not, why. A closed connection is a failure with no reason.
// The pauses login makes: before asking a name again, and after a wrong
// password. Each is on this connection's own goroutine; nobody else waits.
// Variables only so the package's tests can run at their own pace.
var (
	nameAgain     = time.Second
	wrongPassword = 2 * time.Second
)

// pause waits d, or answers false if the connection is closed meanwhile.
func (c *conn) pause(d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-c.quit:
		return false
	}
}

func (c *conn) awaitAuth() (ok bool, why event.ResultCode) {
	select {
	case why := <-c.authResult:
		return why == "", why
	case <-c.quit:
		return false, ""
	}
}

const (
	minPassword   = 8
	maxPassword   = 72 // bytes: bcrypt ignores everything past it
	passwordTries = 3
)

// login is the whole conversation before a player exists: a name, then either
// its password or, for a name nobody has, the offer to create it.
//
// The name goes to the server alone first. A known name comes back as
// PasswordRequired and an unknown one as NoSuchPlayer, which is what decides
// which question comes next.
func (c *conn) login() bool {
	for tries := 0; ; tries++ {
		// Every name is a lookup in the store, on the world's one goroutine:
		// a client pipelining names would have it doing nothing else.
		if tries > 0 && !c.pause(nameAgain) {
			return false
		}
		name, ok := c.prompt("By what name do you wish to be known? ")
		if !ok {
			return false // disconnected
		}
		if isHelp(name) {
			c.Send("Type a name to log in as, or a new one to create a character.\r\n" +
				"Once you're in, type 'help' for the commands.\r\n")
			continue
		}
		c.emit(command.Login{Name: name})
		ok, why := c.awaitAuth()
		switch {
		case ok:
			return true // only a server that asked for no password
		case why == event.AlreadyPlaying:
			c.Send(fmt.Sprintf("%s is already playing.\r\n", name))
		case why == event.InvalidName:
			c.Send("Names are 3 to 16 letters, a to z, and nothing else.\r\n")
		case why == event.NameReserved:
			c.Send("That name belongs to something else here. Pick another.\r\n")
		case why == event.PasswordRequired:
			if done, ok := c.enterPassword(name); done || !ok {
				return done
			}
		case why == event.NoSuchPlayer:
			if done, ok := c.create(name); done || !ok {
				return done
			}
		case why == event.Unknown:
			c.Send("Something went wrong at our end. Try again in a moment.\r\n")
			return false
		default:
			return false // the connection closed while we waited
		}
	}
}

// enterPassword asks for an existing character's password, a few times. done
// is a successful login; !ok means the connection is finished, including
// being hung up on for too many wrong answers. Neither means back to the name.
func (c *conn) enterPassword(name string) (done, ok bool) {
	for range passwordTries {
		password, ok := c.askSecret("Password: ")
		if !ok {
			return false, false
		}
		c.emit(command.Login{Name: name, Password: command.Secret(password)})
		loggedIn, why := c.awaitAuth()
		switch {
		case loggedIn:
			return true, true
		case why == event.BadPassword:
			// a guesser waits, as a person who mistyped hardly notices
			if !c.pause(wrongPassword) {
				return false, false
			}
			c.Send("Wrong password.\r\n")
		case why == event.AlreadyPlaying:
			// someone got in as them while bcrypt was working
			c.Send(fmt.Sprintf("%s is already playing.\r\n", name))
			return false, true
		default:
			return false, false
		}
	}
	c.Send("Too many wrong passwords.\r\n")
	c.Close()
	return false, false
}

// create offers to make a character nobody has, and does. Its results mean
// what enterPassword's do.
func (c *conn) create(name string) (done, ok bool) {
	// the server already found it valid; show it the way it will be stored
	if canonical, err := player.CanonicalName(name); err == nil {
		name = canonical
	}
	if c.mayCreate != nil && !c.mayCreate() {
		c.Send(fmt.Sprintf("No one by the name of %s, and your address has made all the "+
			"new characters it can for today.\r\n", name))
		return false, true
	}
	yn, ok := c.prompt(fmt.Sprintf("No one by the name of %s. Create them? (yn) ", name))
	if !ok {
		return false, false
	}
	if !strings.HasPrefix(strings.ToLower(yn), "y") {
		return false, true
	}
	lineage, ok := c.chooseLineage()
	if !ok {
		return false, false
	}
	password, ok := c.choosePassword()
	if !ok {
		return false, false
	}
	c.emit(command.CreatePlayer{Name: name, Lineage: lineage, Password: command.Secret(password)})
	created, why := c.awaitAuth()
	switch {
	case created:
		if c.created != nil {
			c.created()
		}
		return true, true
	case why == event.NameTaken:
		c.Send(fmt.Sprintf("Someone else has just taken the name %s.\r\n", name))
	case why == "":
		return false, false // closed while we waited
	default:
		c.Send("Something went wrong creating that character.\r\n")
	}
	return false, true
}

// choosePassword asks for a new password twice, until it's acceptable and
// both answers agree.
func (c *conn) choosePassword() (string, bool) {
	for {
		password, ok := c.askSecret("Choose a password: ")
		if !ok {
			return "", false
		}
		if problem := checkPassword(password); problem != "" {
			c.Send(problem + "\r\n")
			continue
		}
		again, ok := c.askSecret("Again: ")
		if !ok {
			return "", false
		}
		if again != password {
			c.Send("Those don't match. Once more.\r\n")
			continue
		}
		return password, true
	}
}

// checkPassword says what's wrong with a new password, or nothing. The
// minimum counts characters, what a player thinks they typed; the maximum
// counts bytes, since the limit is bcrypt's.
func checkPassword(password string) string {
	if utf8.RuneCountInString(password) < minPassword {
		return fmt.Sprintf("A password needs at least %d characters.", minPassword)
	}
	if len(password) > maxPassword {
		return fmt.Sprintf("A password can be at most %d bytes long.", maxPassword)
	}
	return ""
}

// askSecret is prompt with the client's echo off, and back on however it
// ends. The Enter wasn't echoed either, so turning it back on starts a new
// line too.
func (c *conn) askSecret(text string) (string, bool) {
	c.Send(echo(false))
	defer c.Send(echo(true))
	return c.prompt(text)
}

// chooseLineage is the whole of character creation. There is no class step
// after it and no ability step: a lineage decides nothing but how the
// character is described, and what they are good at is decided later, by what
// they pick up and put on.
//
// The player answers with a number or with any unambiguous part of a lineage
// name, and an unrecognized answer re-asks rather than failing the creation.
func (c *conn) chooseLineage() (string, bool) {
	if c.cat == nil {
		return "", true // no catalog: the server picks the default
	}
	choices := lineageChoices(c.cat)
	if len(choices) == 0 {
		return "", true
	}

	c.Send(lineageMenu(c.cat))
	for {
		answer, ok := c.prompt("Which lineage? ")
		if !ok {
			return "", false
		}
		if id, found := matchLineage(choices, answer); found {
			return id, true
		}
		c.Send("That isn't one of them. Type a number, or the name.\r\n")
	}
}

// lineageChoices flattens the species tree into the numbered list the menu
// shows, in content order so the numbers are stable between sessions.
func lineageChoices(cat *rules.Catalog) []*rules.Lineage {
	var out []*rules.Lineage
	for _, s := range cat.SpeciesList() {
		out = append(out, s.Lineages...)
	}
	return out
}

// lineageMenu groups the numbered choices under their species, since the
// species is the only thing the grouping is still for.
func lineageMenu(cat *rules.Catalog) string {
	var b strings.Builder
	b.WriteString("\nChoose a lineage. It decides nothing but how you look;\n")
	b.WriteString("what you're good at comes from what you carry.\n")
	n := 0
	for _, s := range cat.SpeciesList() {
		b.WriteString("\n" + s.Name + "\n")
		for _, l := range s.Lineages {
			n++
			if l.Description != "" {
				fmt.Fprintf(&b, "  %2d) %-20s %s\n", n, l.Name, l.Description)
			} else {
				fmt.Fprintf(&b, "  %2d) %s\n", n, l.Name)
			}
		}
	}
	b.WriteString("\n")
	return b.String()
}

// matchLineage accepts the menu number, the lineage id, or a case-insensitive
// prefix of the name -- but only when exactly one lineage matches it, so
// "h" doesn't silently pick Hill Dwarf over High Elf.
func matchLineage(choices []*rules.Lineage, answer string) (string, bool) {
	answer = strings.TrimSpace(answer)
	if n, err := strconv.Atoi(answer); err == nil {
		if n >= 1 && n <= len(choices) {
			return choices[n-1].Id, true
		}
		return "", false
	}

	lower := strings.ToLower(answer)
	var match *rules.Lineage
	for _, l := range choices {
		if strings.EqualFold(l.Id, answer) || strings.EqualFold(l.Name, answer) {
			return l.Id, true // an exact hit beats any number of prefixes
		}
		if strings.HasPrefix(strings.ToLower(l.Name), lower) {
			if match != nil {
				return "", false // ambiguous
			}
			match = l
		}
	}
	if match == nil {
		return "", false
	}
	return match.Id, true
}

// emit hands a command to the game server.
// It is the only path from this connection into the world.
func (c *conn) emit(cmd command.Command) {
	c.gs.Receive(gameserver.NewHandlerParameter(c, cmd))
}

// prompt writes text with no trailing newline, then waits
// for a reply. A bare Enter re-issues the prompt rather
// than returning an empty string.
func (c *conn) prompt(text string) (string, bool) {
	for {
		if err := c.send(question(text)); err != nil {
			return "", false // queue full; conn is being torn down
		}
		line, ok := c.readLine()
		if !ok {
			return "", false
		}
		if line != "" {
			return line, true
		}
	}
}

// readLine returns the next line from the client. ok is false once the
// connection is finished: EOF, read error, Close.
//
// Each call gets readTimeout to finish, counted from when it starts, so a
// client can't hold the connection open by trickling bytes with no newline.
func (c *conn) readLine() (string, bool) {
	deadline := time.Now().Add(c.readTimeout)
	if !c.loginBy.IsZero() && c.loginBy.Before(deadline) {
		deadline = c.loginBy
	}
	if err := c.netConn.SetReadDeadline(deadline); err != nil {
		return "", false
	}
	if !c.scanner.Scan() {
		return "", false
	}
	return strings.TrimSpace(clean(c.scanner.Text())), true
}

// clean is what a typed line may carry on to the world: printable text.
// Control characters go -- an escape sequence emoted at a room would clear
// its screens -- as do bytes that aren't UTF-8, among them the 0xFF a
// doubled IAC leaves in a line, which echoed back out would be a telnet
// command. A tab is a space.
func clean(line string) string {
	var b strings.Builder
	for i, r := range line {
		switch {
		case r == utf8.RuneError && !strings.HasPrefix(line[i:], string(utf8.RuneError)):
			// an invalid byte, not a real U+FFFD
		case r == '\t':
			b.WriteByte(' ')
		case unicode.IsControl(r), unicode.Is(unicode.Cf, r):
			// Cf too: a right-to-left override in a say reverses the rest of
			// the line on everyone's screen
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (c *conn) Close() {
	c.closeOnce.Do(func() {
		close(c.quit)
		// doesn't close netConn, since the writePump may still
		// have leftover data to send to the client as we're saying
		// goodbye.  we "quit", it drains the buffer, writes, and then
		// netConn.Close().
	})
}

const writeTimeout = 10 * time.Second

func (c *conn) writePump() {
	defer func() {
		// this unblocks a parked readPump
		_ = c.netConn.Close()
		if c.onClose != nil {
			c.onClose()
		}
	}()

	for {
		select {
		case msg := <-c.sendQueue:
			if err := c.write(msg); err != nil {
				log.Warn().Err(err).Msgf("telnet %s write error", c.netConn.RemoteAddr())
				c.Close()
				return
			}
		case <-c.quit:
			// drain what's already queued so the goodbye actually lands
			for {
				select {
				case msg := <-c.sendQueue:
					if err := c.write(msg); err != nil {
						return
					}
				default:
					return
				}
			}
		}
	}
}

func (c *conn) write(msg any) error {
	switch m := msg.(type) {
	case echo:
		return c.writeRaw(echoBytes(bool(m)))
	case negotiation:
		return c.writeRaw(string(m))
	case endOfRecord:
		c.eor = bool(m)
		return nil
	case windowSize:
		c.width = wrapWidth(int(m))
		return nil
	case gmcpOn:
		c.gmcp, c.vitals = bool(m), nil
		return nil
	}
	if c.gmcp {
		var data string
		data, c.vitals = gmcpFor(msg, c.vitals)
		if data != "" {
			if err := c.writeRaw(data); err != nil {
				return err
			}
		}
	}
	text := c.frame(msg)
	if text == "" {
		return nil
	}
	text = strings.ReplaceAll(text, "\r\n", "\n") // normalize
	text = strings.ReplaceAll(text, "\n", "\r\n") // replace with \r\n
	// a literal 0xFF is IAC to the client; never valid UTF-8, so it can only
	// have come from somewhere unclean, and doubled it's just a byte
	text = strings.ReplaceAll(text, "\xff", "\xff\xff")
	switch msg.(type) {
	case event.Prompt, reprompt, question:
		text += c.promptEnd()
	}
	return c.writeRaw(text)
}

// promptEnd marks where a prompt stops. A prompt has no newline, so without
// it a MUD client can only guess -- from a pause -- whether the line it's
// holding is a prompt or the first half of a packet.
func (c *conn) promptEnd() string {
	if c.eor {
		return string([]byte{IAC, EOR})
	}
	return string([]byte{IAC, GA})
}

func (c *conn) writeRaw(text string) error {
	if err := c.netConn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}
	_, err := io.WriteString(c.netConn, text)
	return err
}

// echoBytes is the negotiation for echo. WILL ECHO claims echoing for the
// server, which then doesn't, so nothing typed shows. WONT hands it back,
// and adds the newline the unechoed Enter didn't.
func echoBytes(on bool) string {
	if on {
		return string([]byte{IAC, WONT, optEcho}) + "\r\n"
	}
	return string([]byte{IAC, WILL, optEcho})
}

// frame renders msg and decides where the prompt goes around it. Empty means
// nothing to write.
//
// A prompt is written only when something has been said since the last one,
// since the server sends one after every command and every pulse whether or
// not this player heard anything; and not before login, when the login
// conversation asks its own questions. Anything that arrives while the prompt
// is sitting there unanswered -- a combat round, somebody's shout -- starts on
// a line of its own instead of after the "> ".
func (c *conn) frame(msg any) string {
	if m, ok := msg.(event.Color); ok {
		c.color = m.On // before rendering, so the answer is in the new setting
	}
	text := c.layout(msg)
	if !c.color {
		text = plain(text)
	}
	return wrap(text, c.width)
}

// layout is frame without the color decision.
func (c *conn) layout(msg any) string {
	switch m := msg.(type) {
	case question:
		msg = string(m)
	case inputReceived:
		c.atPrompt = false
		return ""
	case reprompt:
		if c.lastPrompt == nil {
			return ""
		}
		return c.frame(*c.lastPrompt)
	case event.Prompt:
		c.lastPrompt = &m
		if c.atPrompt || c.Player() == nil {
			return ""
		}
		c.atPrompt = true
		return render(m, "")
	}

	name := ""
	if p := c.Player(); p != nil {
		name = p.Name()
	}
	text := render(msg, name)
	if text == "" {
		return ""
	}
	if c.atPrompt {
		text = "\n" + text
	}
	c.atPrompt = false
	return text
}

func (c *conn) readPump() {
	defer c.Close()
	c.scanner = bufio.NewScanner(&iacFilter{
		src:           bufio.NewReader(c.netConn),
		negotiated:    c.negotiated,
		subnegotiated: c.subnegotiated,
	})
	c.readTimeout = c.loginIdle
	c.loginBy = time.Now().Add(loginWithin)
	if c.login() {
		c.loginBy = time.Time{}
		c.readTimeout = c.playIdle
		if c.commandLoop() {
			return // logout already emitted
		}
	}
	cause := "client disconnected"
	if err := c.scanner.Err(); errors.Is(err, os.ErrDeadlineExceeded) {
		cause = "idle"
		c.Send("\r\nIdle too long. Goodbye.\r\n")
	} else if err != nil {
		cause = fmt.Sprintf("read error: %v", err)
	}
	log.Info().Msgf("telnet %s: %s", c.netConn.RemoteAddr(), cause)
	c.gs.Logout(c, cause)
}

// negotiated hears the client's side of option negotiation: EOR, GMCP, and a
// client that stops reporting its window. WILL ECHO is only ever offered around a
// password, and the client's answer changes nothing.
func (c *conn) negotiated(verb, option byte) {
	switch {
	case option == optEOR && verb == DO:
		c.Send(endOfRecord(true))
	case option == optEOR && verb == DONT:
		c.Send(endOfRecord(false))
	case option == optNAWS && verb == WONT:
		c.Send(windowSize(0)) // won't say: back to not wrapping
	case option == optGMCP && verb == DO:
		c.Send(gmcpOn(true))
	case option == optGMCP && verb == DONT:
		c.Send(gmcpOn(false))
	}
}

// subnegotiated hears what a client says about an option. NAWS is the only
// one listened to: width then height, two bytes each. The height is unused.
func (c *conn) subnegotiated(option byte, data []byte) {
	if option == optNAWS && len(data) >= 4 {
		c.Send(windowSize(int(data[0])<<8 | int(data[1])))
	}
}

func (c *conn) commandLoop() (quit bool) {
	for {
		line, ok := c.readLine()
		if !ok {
			return false
		}
		c.Send(inputReceived{})
		if line == "" {
			// bare Enter: nothing to do but ask again. The world never hears
			// about it, so the prompt has to come from here.
			c.Send(reprompt{})
			continue
		}
		if text, ok := helpFor(line); ok {
			// the world never hears it, so the prompt has to come from here
			c.Send(text)
			c.Send(reprompt{})
			continue
		}
		cmd, err := parseCommand(strings.Fields(line))
		if err != nil {
			c.Send(err.Error() + "\r\n")
			c.Send(reprompt{}) // likewise, the world never saw it
			continue
		}
		c.emit(cmd)
		if _, quitting := cmd.(command.Logout); quitting {
			c.Send("Goodbye.\r\n")
			return true
		}
	}
}

// addressLimit counts open connections per remote host. The accept loop
// acquires and each connection's writePump releases, so it needs its lock.
type addressLimit struct {
	mu    sync.Mutex
	max   int
	open  map[string]int
	total int // every connection; refused past maxConns
	// made is when each address last created characters, within the last
	// createWindow: names are forever, and each is a document in the store.
	made map[string][]time.Time
}

// An address may make createsPerWindow characters in any createWindow --
// a household's worth, not a script's.
const (
	createsPerWindow = 10
	createWindow     = 24 * time.Hour
)

// mayCreate is whether host may make another character now.
func (l *addressLimit) mayCreate(host string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(host, now)) < createsPerWindow
}

// created counts a character host made.
func (l *addressLimit) created(host string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.made == nil {
		l.made = map[string][]time.Time{}
	}
	l.made[host] = append(l.recent(host, now), now)
}

// recent is host's creations still inside the window, and forgets the rest
// -- and the address, once it has none. Called with mu held.
func (l *addressLimit) recent(host string, now time.Time) []time.Time {
	kept := slices.DeleteFunc(l.made[host], func(t time.Time) bool { return now.Sub(t) >= createWindow })
	if len(kept) == 0 {
		delete(l.made, host)
	} else {
		l.made[host] = kept
	}
	return kept
}

// acquire takes a slot for host, or answers why not: full is everyone
// together, otherwise it's this host.
func (l *addressLimit) acquire(host string) (ok, full bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.total >= maxConns {
		return false, true
	}
	if l.open[host] >= l.max {
		return false, false
	}
	l.open[host]++
	l.total++
	return true, false
}

func (l *addressLimit) release(host string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.total--
	if l.open[host]--; l.open[host] <= 0 {
		delete(l.open, host) // or the map grows by every address that ever connected
	}
}

// remoteHost is the IP a connection came from, without the port, which is
// different for every connection and would make the limit count nothing.
func remoteHost(nc net.Conn) string {
	host, _, err := net.SplitHostPort(nc.RemoteAddr().String())
	if err != nil {
		return nc.RemoteAddr().String()
	}
	return hostKey(host)
}

// hostKey is what one address counts as: an IPv4 address itself, and an
// IPv6 one by its /64, the block a single customer is handed -- counted one
// by one, they would be 2^64 different addresses.
func hostKey(host string) string {
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() != nil {
		return host
	}
	return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

// refuse says why and hangs up, before a conn or its goroutines exist.
func refuse(nc net.Conn, why string) {
	_ = nc.SetWriteDeadline(time.Now().Add(writeTimeout))
	_, _ = io.WriteString(nc, why)
	_ = nc.Close()
}
