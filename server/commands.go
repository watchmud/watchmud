package server

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/player"
)

// loginChecked is a command.Command that contains the callback from comparing a password to the stored password hash,
// not exported and put over here so clients can't ever forge one.
type loginChecked struct {
	Name string
	Ok   bool
	// Rec is the record the password was checked against, loaded off the
	// world goroutine, and Gen the name's logout count when it was (see
	// GameServer.logouts): a logout since makes Rec stale.
	Rec *player.Record
	Gen int
}

// loginLooked is the store's answer about a name, from a goroutine: a store
// is a database, and the world goroutine mustn't wait on one. Password is
// what the login carried, to check once the record's in hand.
type loginLooked struct {
	Name     string
	Password command.Secret
	Rec      *player.Record
	Found    bool
	Err      error
	Gen      int
}

func (loginLooked) Verb() string { return "loginLooked" }

func (loginChecked) Verb() string { return "loginChecked" }

// createHashed is a command.Command that creates a hashed password from a plaintext password when
// creating a new player and record. Private for the same reason as loginChecked.
type createHashed struct {
	Name         string
	HashPassword command.Secret
	// Lineage is a rules.Lineage id, and the only choice creation makes. It
	// is cosmetic: there is no class to pick beside it, because what a
	// character is good at comes from the gear they put on.
	Lineage string
	// Taken is the store having the name already; Err the store or the hash
	// failing. Either way nothing is created.
	Taken bool
	Err   error
}

func (createHashed) Verb() string { return "createHashed" }
