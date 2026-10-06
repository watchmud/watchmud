package server

import (
	"fmt"
	"net/http"
	"time"
)

// LastBeat is when Run last finished a tick: the zero time before Run has
// started. It is the one thing about the server that is safe to read from
// another goroutine -- an atomic, not world state.
//
// A tick only runs when the loop gets back to its select, so a handler or a
// pulse that never returns stops this moving while the listeners carry on
// accepting. That wedge is what a health check is for, and why it reads this
// rather than opening a telnet session.
func (gs *GameServer) LastBeat() time.Time {
	n := gs.lastBeat.Load()
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}

func (gs *GameServer) beat(now time.Time) {
	gs.lastBeat.Store(now.UnixNano())
}

// HealthHandler answers 200 while the loop has ticked within maxAge, and 503
// otherwise, including before Run has started. The body says how long ago,
// for whoever is reading it by hand.
func (gs *GameServer) HealthHandler(maxAge time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		last := gs.LastBeat()
		if last.IsZero() {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, "starting: no heartbeat yet")
			return
		}
		age := time.Since(last).Round(time.Millisecond)
		if age > maxAge {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "stale: last heartbeat %s ago\n", age)
			return
		}
		fmt.Fprintf(w, "ok: last heartbeat %s ago\n", age)
	})
}
