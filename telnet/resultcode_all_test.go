package telnet

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every result code the world can fail with has words of its own, keyed on
// the code itself or on some verb and the code. A code left out reads as "You
// can't do that.", which is how a whole set of bag messages went unseen: they
// had been filed under per-verb keys with no verb.
func TestFailureText_everyCodeHasWords(t *testing.T) {
	src, err := os.ReadFile("../event/result.go")
	require.NoError(t, err)
	codes := regexp.MustCompile(`ResultCode = "([A-Z_]+)"`).FindAllStringSubmatch(string(src), -1)
	require.NotEmpty(t, codes)

	verbKey := regexp.MustCompile(`^[a-z]+/([A-Z_]+)$`)
	byVerb := map[string]bool{}
	for k := range failureByVerb {
		m := verbKey.FindStringSubmatch(k)
		require.NotNil(t, m, "failureByVerb key %q isn't verb/CODE", k)
		byVerb[m[1]] = true
	}
	// the login conversation words these itself (conn.go), as answers to a
	// question rather than failures of a command
	login := map[string]bool{"PLAYER_LOGIN_FAILED": true, "ALREADY_PLAYING": true, "BAD_PASSWORD": true,
		"PASSWORD_REQUIRED": true, "INVALID_NAME": true, "NAME_RESERVED": true, "NAME_TAKEN": true}
	for _, c := range codes {
		code := c[1]
		if login[code] {
			continue
		}
		_, plain := failureByCode[code]
		prefixed := false
		for prefix := range failureByPrefix {
			if len(code) >= len(prefix) && code[:len(prefix)] == prefix {
				prefixed = true
			}
		}
		assert.True(t, plain || byVerb[code] || prefixed || code == "UNKNOWN_COMMAND", "%s has no words", code)
	}
}
