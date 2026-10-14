package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/player"
)

func checkHealth(gs *GameServer, maxAge time.Duration) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	gs.HealthHandler(maxAge).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	return rec
}

func TestHealth_notStarted(t *testing.T) {
	gs, _ := newTestGameServer(t)

	rec := checkHealth(gs, time.Minute)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "no heartbeat yet")
}

func TestHealth_stale(t *testing.T) {
	gs, _ := newTestGameServer(t)
	gs.beat(time.Now().Add(-time.Minute))

	rec := checkHealth(gs, 30*time.Second)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "stale")
}

// The loop ticking is what keeps it healthy, and the loop stopping -- here
// by being cancelled, in life by being wedged -- is what makes it stale.
func TestHealth_followsTheLoop(t *testing.T) {
	gs, _ := newTestGameServer(t)
	gs.SetTickInterval(5 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = gs.Run(ctx)
		close(done)
	}()

	require.Eventually(t, func() bool {
		return checkHealth(gs, 100*time.Millisecond).Code == http.StatusOK
	}, 5*time.Second, 5*time.Millisecond)
	assert.Contains(t, checkHealth(gs, 100*time.Millisecond).Body.String(), "ok")

	cancel()
	<-done
	require.Eventually(t, func() bool {
		return checkHealth(gs, 100*time.Millisecond).Code == http.StatusServiceUnavailable
	}, 5*time.Second, 10*time.Millisecond)
}

// What a listing site is told: people, not bots, as of the last prompt.
func TestPlaying_countsPeopleNotBots(t *testing.T) {
	gs, _ := newTestGameServer(t)
	assert.Equal(t, 0, gs.Playing())

	ann := player.NewTestPlayer(uuid.New(), "ann", nil)
	bot := player.NewTestPlayer(uuid.New(), "botty", nil)
	bot.SetBot(true)
	gs.world.PlacePlayer(ann, gs.world.StartRoom)
	gs.world.PlacePlayer(bot, gs.world.StartRoom)
	gs.prompt()
	assert.Equal(t, 1, gs.Playing())
	assert.False(t, gs.Started().IsZero())
}
