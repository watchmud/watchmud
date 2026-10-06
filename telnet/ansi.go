package telnet

import "regexp"

// The renderer always colors, and conn.frame strips it for a player who has
// color off: one place decides what is which color, and the renderer never
// asks who it's talking to about anything but their name.
//
// Only the SGR sequences below ever appear, so stripping is one regexp.
const (
	reset = "\x1b[0m"

	bold    = "\x1b[1m"
	dim     = "\x1b[2m"
	red     = "\x1b[31m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	magenta = "\x1b[35m"
	cyan    = "\x1b[36m"

	boldRed    = "\x1b[1;31m"
	boldYellow = "\x1b[1;33m"
	boldCyan   = "\x1b[1;36m"
)

// What each kind of thing is. Change the look here, not in render.go.
const (
	colorRoomName = boldCyan
	colorExits    = cyan
	colorObject   = green
	colorMob      = yellow
	colorPlayer   = bold
	colorBot      = dim // a bot's name in who: there, but not a person
	colorRole     = magenta
	colorPlace    = cyan // where someone is, in who
	colorHeading  = bold
	colorAbility  = boldCyan // an ability's name, in the abilities list
	colorReady    = green    // an ability that can be cast now
	colorWaiting  = yellow   // one still cooling down
	colorCoins    = yellow

	colorSay   = bold
	colorTell  = magenta
	colorShout = boldYellow
	colorOOC   = cyan // the ooc channel: not in the world, so not in its colors

	colorHurt   = red     // a blow that landed on you
	colorDeath  = boldRed // you died, or something did
	colorBroken = red     // gear giving out

	colorHealthy = green
	colorWounded = yellow
	colorDying   = red
)

// paint wraps s in a color. Each piece resets itself, so a stripped or
// truncated line never leaves the terminal colored.
func paint(color, s string) string {
	if s == "" {
		return s
	}
	return color + s + reset
}

// healthColor is the prompt's warning: green above two thirds, yellow above
// a third, red below.
func healthColor(cur, max int) string {
	switch {
	case max <= 0 || cur*3 > max*2:
		return colorHealthy
	case cur*3 > max:
		return colorWounded
	default:
		return colorDying
	}
}

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// plain removes the color, for a player who doesn't want it.
func plain(s string) string {
	return sgr.ReplaceAllString(s, "")
}
