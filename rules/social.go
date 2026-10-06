package rules

import (
	"fmt"
	"slices"
	"strings"
)

// Social is a gesture with no effect but what people read: "smile", "bow
// bob". Content, from rules/socials.json. In each line $n is the one doing it
// and $N the one it's done to.
type Social struct {
	Name string `json:"name"`
	// Alone is with no target: what the doer reads, and the room.
	Alone SocialLines `json:"alone"`
	// At is with a target, a player or a mob in the room; nil means it
	// takes none. Victim is what the target reads, if it's a player.
	At *SocialLines `json:"at"`
}

type SocialLines struct {
	Self   string `json:"self"`
	Victim string `json:"victim,omitempty"`
	Room   string `json:"room"`
}

// Fill puts the doer's and the target's names into a line.
func (SocialLines) Fill(line, actor, target string) string {
	return strings.NewReplacer("$n", actor, "$N", target).Replace(line)
}

// SetSocials checks and indexes socials: a lowercase name of letters, no
// name twice, both alone lines, and all three at a target if it takes one.
func (c *Catalog) SetSocials(all []*Social) error {
	byName := map[string]*Social{}
	for _, s := range all {
		if s.Name == "" || strings.ToLower(s.Name) != s.Name || strings.ContainsFunc(s.Name, func(r rune) bool { return r < 'a' || r > 'z' }) {
			return fmt.Errorf("social %q: a name is lowercase letters", s.Name)
		}
		if byName[s.Name] != nil {
			return fmt.Errorf("social %q twice", s.Name)
		}
		if s.Alone.Self == "" || s.Alone.Room == "" {
			return fmt.Errorf("social %s: alone needs self and room", s.Name)
		}
		if s.At != nil && (s.At.Self == "" || s.At.Victim == "" || s.At.Room == "") {
			return fmt.Errorf("social %s: at needs self, victim and room", s.Name)
		}
		byName[s.Name] = s
	}
	c.Socials = byName
	c.socialOrder = slices.Clone(all)
	return nil
}

// SocialList is every social, in the order content declared them.
func (c *Catalog) SocialList() []*Social { return c.socialOrder }
