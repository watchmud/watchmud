package loader

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/watchmud/watchmud/script"
)

// mobScript resolves a mob's "script" -- bare for its own zone, "zone/name"
// for any other, the same rule loot uses -- and compiles it the first time
// anything names it. It returns the canonical "zone/name", empty for no
// script. Anything wrong fails startup: a script that can't load would
// otherwise be a mob that silently never speaks. Only named scripts are
// compiled; a .lua nothing names is ignored.
func (c *Content) mobScript(fsys fs.FS, zoneName string, mob mobEntry) (string, error) {
	if mob.Script == "" {
		return "", nil
	}
	zoneId, name := zoneName, mob.Script
	if z, n, found := strings.Cut(mob.Script, "/"); found {
		zoneId, name = z, n
	}
	if name == "" {
		return "", fmt.Errorf("mob %s/%s: script %q names no script", zoneName, mob.Id, mob.Script)
	}
	if _, ok := c.Zones[zoneId]; !ok {
		return "", fmt.Errorf("mob %s/%s: script %q: zone not found", zoneName, mob.Id, mob.Script)
	}
	ref := zoneId + "/" + name
	if _, done := c.Scripts[ref]; done {
		return ref, nil
	}
	file := path.Join(zoneId, "scripts", name+".lua")
	src, err := fs.ReadFile(fsys, file)
	if err != nil {
		return "", fmt.Errorf("mob %s/%s: script %q: %w", zoneName, mob.Id, mob.Script, err)
	}
	p, err := script.Compile(ref, string(src))
	if err != nil {
		return "", fmt.Errorf("mob %s/%s: %w", zoneName, mob.Id, err)
	}
	c.Scripts[ref] = p
	return ref, nil
}
