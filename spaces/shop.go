package spaces

import "github.com/watchmud/watchmud/object"

// Shop is a room that trades: it sells its Stock, and buys anything worth
// something. Static content, from a zone's shops.json -- what a shop has for
// sale never runs out and never changes while the server runs.
type Shop struct {
	Stock []ShopItem
}

// ShopItem is something a shop sells, and the power it's made at: a shop's
// stock is new, so its power is the shop's to choose, not a roll.
type ShopItem struct {
	Object *object.Definition
	Power  int
}
