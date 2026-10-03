package telnet

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/command"
)

const maxHelpLines = 60

// Help can't advertise a verb the parser doesn't know -- that is how the two
// drift apart, and a new player's first try is whatever help told them.
func TestHelp_everyVerbParses(t *testing.T) {
	for _, section := range helpSections {
		for _, e := range section.entries {
			require.NotEmpty(t, e.verbs, "%q lists no verbs to check", e.usage)
			for _, verb := range e.verbs {
				cmd, err := parseCommand([]string{verb, "x", "y"})
				require.NoError(t, err, "help offers %q", verb)
				assert.NotNil(t, cmd, verb)
			}
		}
	}
}

// Builder commands stay out of it: to a player they don't exist.
func TestHelp_noWizardCommands(t *testing.T) {
	for _, section := range helpSections {
		for _, e := range section.entries {
			for _, verb := range e.verbs {
				cmd, _ := parseCommand([]string{verb, "x", "y"})
				_, wiz := cmd.(command.Wizard)
				assert.False(t, wiz, "help offers the builder command %q", verb)
			}
		}
	}
}

// One screen, on the narrowest terminal anyone still uses.
func TestHelp_fitsOneScreen(t *testing.T) {
	lines := strings.Split(strings.TrimRight(helpText, "\n"), "\n")
	assert.LessOrEqual(t, len(lines), maxHelpLines)
	for _, l := range lines {
		assert.LessOrEqual(t, utf8.RuneCountInString(l), 79, "%q", l)
	}
}

// help and ? are answered by the connection; the world never hears them.
func TestHelp_inGame(t *testing.T) {
	gs := &fakeServer{passwords: map[string]string{"Bob": "sekrit99"}}
	s := startSession(t, gs)
	s.answer("known? ", "Bob")
	s.answer("Password: ", "sekrit99")
	s.answer(echoOn, "help")
	s.waitFor("Moving around")
	s.answer("save and leave", "?")
	require.Eventually(t, func() bool {
		return strings.Count(s.transcript(), "Moving around") == 2
	}, 2*time.Second, time.Millisecond)

	assert.Len(t, gs.commands(), 2, "only the two logins reached the server")
}

// At the name prompt, "help" is a question, not a character called Help.
func TestHelp_atTheNamePrompt(t *testing.T) {
	gs := &fakeServer{passwords: map[string]string{}}
	s := startSession(t, gs)
	s.answer("known? ", "help")
	s.waitFor("Type a name")

	assert.Empty(t, gs.commands(), "no login was tried for Help")
}

func TestShoutIsTellAll(t *testing.T) {
	cmd, err := parseCommand([]string{"shout", "hello", "all"})
	require.NoError(t, err)
	assert.Equal(t, command.TellAll{Value: "hello all"}, cmd)
}

// help takes a topic, for what isn't a command -- the first is who the bots
// are -- and the command list says which topics there are.
func TestHelp_topics(t *testing.T) {
	text, ok := helpFor("help bots")
	require.True(t, ok)
	assert.Contains(t, text, "programs, not people")

	again, ok := helpFor("  HELP   Bots ")
	require.True(t, ok)
	assert.Equal(t, text, again)

	text, ok = helpFor("help dragons")
	require.True(t, ok, "an unknown topic is still help, not a command")
	assert.Equal(t, "There's no help on that. Type 'help' for the commands.\n", text)

	_, ok = helpFor("helpful")
	assert.False(t, ok)

	assert.Contains(t, helpText, "help bots")
	for name, topic := range helpTopics {
		for _, l := range strings.Split(strings.TrimRight(topic.text, "\n"), "\n") {
			assert.LessOrEqual(t, utf8.RuneCountInString(l), 79, "help %s: %q", name, l)
		}
	}
}

func TestHelp_topicInGame(t *testing.T) {
	gs := &fakeServer{passwords: map[string]string{"Bob": "sekrit99"}}
	s := startSession(t, gs)
	s.answer("known? ", "Bob")
	s.answer("Password: ", "sekrit99")
	s.answer(echoOn, "help bots")
	s.waitFor("programs, not people")
	assert.Len(t, gs.commands(), 2, "only the login (name, then password) reached the server")
}
