// Package event is the game's outbound vocabulary: plain structs describing
// what happened, sent to players and mobiles.
//
// Two rules keep this package honest:
//
//   - It is a leaf. It imports direction, slot and the standard library, and
//     nothing else. No *player.Player, no *spaces.Room, no *object.Instance in
//     a field -- only strings, ints and typed enums. spaces imports event, so
//     anything more would be a cycle waiting to happen.
//   - A success event carries its payload and nothing else. There is no
//     Success bool and no ResultCode; failure is Failed, below.
package event

// ResultCode says why a command could not be carried out.
//
// The values are the strings world/ has always emitted, so
// telnet/resultcode.go -- which maps them to player-facing text -- keeps
// working unchanged.
type ResultCode string

const (
	// I dunno
	Unknown    ResultCode = "UNKNOWN"
	BadRequest ResultCode = "BAD_REQUEST"

	// targets
	TargetNotFound    ResultCode = "TARGET_NOT_FOUND"
	TargetNotGettable ResultCode = "TARGET_NOT_GETTABLE"
	NoTarget          ResultCode = "NO_TARGET"

	// containers
	NotAContainer  ResultCode = "NOT_A_CONTAINER"
	NotInContainer ResultCode = "NOT_IN_CONTAINER"
	ContainerEmpty ResultCode = "CONTAINER_EMPTY"

	// carrying and wearing
	TargetInUse   ResultCode = "TARGET_IN_USE"
	InUse         ResultCode = "IN_USE"
	LocationInUse ResultCode = "LOCATION_IN_USE"
	CantWearThat  ResultCode = "CANT_WEAR_THAT"
	CantWearThere ResultCode = "CANT_WEAR_THERE"
	NoSlotGiven   ResultCode = "NO_SLOT_GIVEN"
	NoSmith       ResultCode = "NO_SMITH"
	NoShop        ResultCode = "NO_SHOP"
	NotForSale    ResultCode = "NOT_FOR_SALE"
	Worthless     ResultCode = "WORTHLESS"
	NotDamaged    ResultCode = "NOT_DAMAGED"

	// abilities
	UnknownAbility ResultCode = "UNKNOWN_ABILITY"
	NotGranted     ResultCode = "NOT_GRANTED"
	NotReady       ResultCode = "NOT_READY"
	NotEnoughMana  ResultCode = "NOT_ENOUGH_MANA"
	NoFoe          ResultCode = "NO_FOE"

	// doors
	NoDoor        ResultCode = "NO_DOOR"
	DoorShut      ResultCode = "DOOR_CLOSED"
	AlreadyOpen   ResultCode = "ALREADY_OPEN"
	AlreadyClosed ResultCode = "ALREADY_CLOSED"
	AlreadyLocked ResultCode = "ALREADY_LOCKED"
	NotLocked     ResultCode = "NOT_LOCKED"
	Locked        ResultCode = "LOCKED"
	NotClosed     ResultCode = "NOT_CLOSED"
	NoKey         ResultCode = "NO_KEY"
	NoKeyhole     ResultCode = "NO_KEYHOLE"

	// movement and combat
	CantGoThatWay   ResultCode = "CANT_GO_THAT_WAY"
	NoFightRoom     ResultCode = "NO_FIGHT_ROOM"
	NoFight         ResultCode = "NO_FIGHT"
	InAFight        ResultCode = "IN_A_FIGHT"
	AlreadyFighting ResultCode = "ALREADY_FIGHTING"
	CantFlee        ResultCode = "CANT_FLEE"

	// talking
	ToPlayerNotFound ResultCode = "TO_PLAYER_NOT_FOUND"
	NoValue          ResultCode = "NO_VALUE"

	// nowhere at all. Three spellings of one broken state, kept because
	// telnet/resultcode.go already renders all three the same way.
	NotInRoom        ResultCode = "NOT_IN_ROOM"
	NotInARoom       ResultCode = "NOT_IN_A_ROOM"
	YouAreNotInARoom ResultCode = "YOU_ARE_NOT_IN_A_ROOM"

	// builder commands: the audience is someone editing content/
	UnknownType         ResultCode = "UNKNOWN_TYPE"
	UnknownZone         ResultCode = "UNKNOWN_ZONE"
	UnknownId           ResultCode = "UNKNOWN_ID"
	UnknownDefinitionId ResultCode = "UNKNOWN_DEFINITION_ID"

	// internal failures: the player did nothing wrong
	AddToRoomError         ResultCode = "ADD_TO_ROOM_ERROR"
	RemoveFromRoomError    ResultCode = "REMOVE_FROM_ROOM_ERROR"
	AddRoomInventoryFailed ResultCode = "ADD_ROOM_INVENTORY_FAILED"
	DataError              ResultCode = "DATA_ERROR"
	InternalError          ResultCode = "INTERNAL_ERROR"

	// the parser or the dispatcher, not a handler
	ParseError     ResultCode = "PARSE_ERROR"
	UnknownCommand ResultCode = "UNKNOWN_COMMAND"

	// login. This reaches telnet/conn.go's login(), not the renderer: it is
	// what drives the "No one by that name. Create them?" prompt.
	NoSuchPlayer ResultCode = "PLAYER_LOGIN_FAILED"
	// AlreadyPlaying: that character is in the world on another connection.
	// Two sessions of one character each save over the other, which is how
	// items get duplicated.
	AlreadyPlaying ResultCode = "ALREADY_PLAYING"
	// BadPassword: the character exists and that isn't its password. It must
	// never be empty -- login() reads an empty reason as success.
	BadPassword ResultCode = "BAD_PASSWORD"
	// PasswordRequired: the character exists and the login carried no
	// password. It is how the login conversation learns to ask for one
	// rather than offer to create the character.
	PasswordRequired ResultCode = "PASSWORD_REQUIRED"
	// InvalidName: not a name at all -- 3 to 16 letters a-z and nothing else.
	InvalidName ResultCode = "INVALID_NAME"
	// NameReserved: a word the world already uses, a mob's or the target
	// grammar's, that a player's name would collide with.
	NameReserved ResultCode = "NAME_RESERVED"
	// NameTaken: creation lost a race for the name, or was asked for one that
	// exists. The store no longer says so itself -- saves are queued, and the
	// database's unique index is only heard from on the writer goroutine.
	NameTaken ResultCode = "NAME_TAKEN"
)

// Failed is what a command produces when it cannot be carried out.
//
// Verb is the command the player typed. It is here because a code can mean
// different things to different commands: TARGET_NOT_FOUND is "you don't see
// that here" to get, and "you aren't carrying that" to drop.
type Failed struct {
	Verb string
	Code ResultCode
}
