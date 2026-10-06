// Package mudlet builds WatchMUD's Mudlet package: watchmud.lua, wrapped the
// way Mudlet installs a package -- a zip holding config.lua, which names it,
// and an XML file of the same name holding the script. Players install it
// from www.watchmud.com (site/, built by .github/workflows/pages.yaml).
package mudlet

import (
	"archive/zip"
	_ "embed"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Name is the package's name in Mudlet: what config.lua says and what the
// XML file inside is called.
const Name = "WatchMUD"

//go:embed watchmud.lua
var Script string

// The XML Mudlet reads a package's scripts from. Only what a script needs:
// Mudlet fills in the rest with defaults.
type mudletPackage struct {
	XMLName       xml.Name      `xml:"MudletPackage"`
	Version       string        `xml:"version,attr"`
	ScriptPackage scriptPackage `xml:"ScriptPackage"`
}

type scriptPackage struct {
	Scripts []script `xml:"Script"`
}

type script struct {
	IsActive      string   `xml:"isActive,attr"`
	IsFolder      string   `xml:"isFolder,attr"`
	Name          string   `xml:"name"`
	PackageName   string   `xml:"packageName"`
	Script        string   `xml:"script"`
	EventHandlers []string `xml:"eventHandlerList>string"`
}

// version is the script's own W.version, so the package and what it installs
// can't disagree.
func version() string {
	const marker = `W.version = "`
	i := strings.Index(Script, marker)
	if i < 0 {
		return "0"
	}
	rest := Script[i+len(marker):]
	return rest[:strings.Index(rest, `"`)]
}

// Build writes the .mpackage to w.
func Build(w io.Writer) error {
	z := zip.NewWriter(w)

	cfg, err := z.Create("config.lua")
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(cfg, "mpackage = %q\nauthor = %q\ntitle = %q\ndescription = %q\nversion = %q\n",
		Name, "WatchMUD", "WatchMUD bars and map",
		"Health and mana bars, and a map that draws itself as you walk, from the game's GMCP.",
		version()); err != nil {
		return err
	}

	body, err := xml.MarshalIndent(mudletPackage{
		Version: "1.001",
		ScriptPackage: scriptPackage{Scripts: []script{{
			IsActive: "yes",
			IsFolder: "no",
			Name:     Name,
			Script:   Script,
		}}},
	}, "", "  ")
	if err != nil {
		return err
	}
	pkg, err := z.Create(Name + ".xml")
	if err != nil {
		return err
	}
	if _, err := io.WriteString(pkg, xml.Header+"<!DOCTYPE MudletPackage>\n"); err != nil {
		return err
	}
	if _, err := pkg.Write(body); err != nil {
		return err
	}
	return z.Close()
}
