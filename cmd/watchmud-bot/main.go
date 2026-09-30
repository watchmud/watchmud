// watchmud-bot is the smoke test deploy.sh runs after a restart: it logs in
// as an existing character, walks to the millpond, fights, loots and quits.
//
//	WATCHMUD_BOT_PASSWORD=... watchmud-bot -addr watchmud.com:4000 -name Tester
//
// The password is only ever the environment, never a flag, so it isn't in ps
// or anybody's shell history.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/watchmud/watchmud/bot"
)

func main() {
	os.Exit(run())
}

func run() int {
	addr := flag.String("addr", "localhost:4000", "the game's telnet address")
	name := flag.String("name", "Tester", "the character to log in as; it must already exist")
	wait := flag.Duration("wait", 60*time.Second, "how long to keep trying to connect")
	flag.Parse()

	password := os.Getenv("WATCHMUD_BOT_PASSWORD")
	if password == "" {
		fmt.Fprintln(os.Stderr, "watchmud-bot: set WATCHMUD_BOT_PASSWORD")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *wait)
	defer cancel()
	c, err := bot.Dial(ctx, *addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "watchmud-bot: %v\n", err)
		return 1
	}
	defer c.Close()

	res, err := bot.Smoke(c, bot.Config{Name: *name, Password: password}, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "watchmud-bot: FAILED: %v\n\n--- transcript ---\n%s\n", err, c.Transcript())
		return 1
	}
	for _, n := range res.Notes {
		fmt.Printf("note: %s\n", n)
	}
	fmt.Println("watchmud-bot: passed")
	return 0
}
