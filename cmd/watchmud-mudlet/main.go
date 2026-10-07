// watchmud-mudlet writes WatchMUD's Mudlet package, for the site to serve:
//
//	go run ./cmd/watchmud-mudlet -o site/watchmud.mpackage
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/watchmud/watchmud/mudlet"
)

func main() {
	out := flag.String("o", "watchmud.mpackage", "where to write the package")
	flag.Parse()
	f, err := os.Create(*out)
	if err == nil {
		err = mudlet.Build(f)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "watchmud-mudlet: %v\n", err)
		os.Exit(1)
	}
}
