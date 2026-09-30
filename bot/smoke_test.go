package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSmoke_againstTheRealWorld(t *testing.T) {
	addr := startGame(t, 10*time.Millisecond)
	createCharacter(t, addr, "Tester", "correcthorse")

	c := dial(t, addr)
	var log strings.Builder
	res, err := Smoke(c, Config{Name: "Tester", Password: "correcthorse"}, &log)
	t.Log("\n" + log.String())
	require.NoError(t, err, "%s\n--- transcript ---\n%s", log.String(), c.Transcript())
	assert.Empty(t, res.Notes, "a fresh world has geese, so the fight must have been tested")
}
