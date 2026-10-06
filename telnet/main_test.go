package telnet

import (
	"os"
	"testing"
	"time"
)

// The login conversation's pauses are for guessers and floods; a test that
// mistypes on purpose shouldn't sit through them.
func TestMain(m *testing.M) {
	nameAgain, wrongPassword = time.Millisecond, time.Millisecond
	os.Exit(m.Run())
}
