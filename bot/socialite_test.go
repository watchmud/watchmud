package bot

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopicFor(t *testing.T) {
	for q, want := range map[string]string{
		"how do I get home?":        "recall",
		"where should I hunt?":      "hunt",
		"my shirt broke, now what?": "repair",
		"what's a tank?":            "role",
		"how do i get health back":  "heal",
		"Where can I buy a sword?":  "shop",
	} {
		got := topicFor(q)
		if assert.NotNil(t, got, q) {
			assert.Contains(t, got.words, want, "%q answered with %q", q, got.answer)
		}
	}
	assert.Nil(t, topicFor("nice weather"))
	assert.Equal(t, menu, answer("what's your favourite colour?"))
}

func socialite(t *testing.T) (*Adventurer, *pipeGame) {
	t.Helper()
	a, g := adventurer(t)
	a.cfg.Style = Socialite
	a.here = home
	return a, g
}

// says is what react says in the room.
func says(a *Adventurer, text string) []string {
	out, _ := a.react(text)
	return out
}

// oocs is what react answers on the ooc channel.
func oocs(a *Adventurer, text string) []string {
	_, out := a.react(text)
	return out
}

// A first-timer is welcomed once; a returning player isn't welcomed at all.
func TestReact_welcomesNewCharactersOnce(t *testing.T) {
	a, _ := socialite(t)

	lines := says(a, "Bob has entered the game for the first time.\nAnn has entered the game.\n")
	assert.Equal(t, []string{welcomeFor("Bob")}, lines)
	assert.Empty(t, says(a, "Bob has entered the game for the first time.\n"), "once")
}

// What it answers, said in the room: a question it has an answer to, anything
// with its name or asking for help -- not a question for somebody else, not a
// bot, and not the same person over and over.
func TestReact_answersWhatsPutToIt(t *testing.T) {
	a, _ := socialite(t)
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }
	recall := topicFor("recall").answer

	assert.Equal(t, []string{recall}, says(a, `Bob says, "how do I recall?".`+"\n"))
	assert.Empty(t, says(a, `Ann says, "how was your day?".`+"\n"), "somebody else's conversation")
	assert.Equal(t, []string{menu}, says(a, `Ann says, "Testbot, what's good?".`+"\n"), "by name")
	assert.Empty(t, says(a, `Pim says, "Back in town. Anything exciting happen?".`+"\n"), "a sibling")
	assert.Empty(t, says(a, `Bob says, "and repairs?".`+"\n"), "not again so soon")

	now = now.Add(answerEvery)
	assert.Len(t, says(a, `Bob says, "and repairs?".`+"\n"), 1)
}

// On ooc it answers a question it has a topic for, or its name, addressed to
// the asker -- never a menu to a question that isn't for it, never itself.
func TestReact_answersOnOOC(t *testing.T) {
	a, _ := socialite(t)
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }

	assert.Equal(t, []string{"Bob: " + topicFor("recall").answer}, oocs(a, "[ooc] Bob: how do I recall?\n"))
	assert.Empty(t, says(a, "[ooc] Ann: how do I repair?\n"), "answered on ooc, not said in the room")
	assert.Empty(t, oocs(a, "[ooc] Dot: anyone seen Pim today?\n"), "no menu for a question that isn't for it")
	assert.Empty(t, oocs(a, "[ooc] Testbot: Bob: ...\n"), "its own line")
	assert.Empty(t, oocs(a, "[ooc] Bob: and repairs?\n"), "not again so soon")
	assert.Equal(t, []string{"Cal: " + menu}, oocs(a, "[ooc] Cal: Testbot, you there?\n"), "by name")
}

// The loop: a welcome said, a tell answered by tell.
func TestSocialize_welcomesAndAnswersTells(t *testing.T) {
	a, g := socialite(t)
	go func() { _, _ = a.socialize(context.Background()) }()

	g.say("Bob has entered the game for the first time.\r\n<100/100hp> ")
	assert.Equal(t, "say "+welcomeFor("Bob"), g.heard())
	g.say("You say, \"" + welcomeFor("Bob") + "\".\r\n<100/100hp> ")

	g.say("Bob tells you, \"how do I heal?\".\r\n<100/100hp> ")
	assert.Equal(t, "tell Bob "+topicFor("heal").answer, g.heard())
}

// Every command an answer tells someone to type is one the game knows: sent
// to the real game, not one comes back as an unknown request.
func TestFAQ_commandsTheGameKnows(t *testing.T) {
	var said []string
	for _, tp := range faq {
		said = append(said, tp.answer)
	}
	said = append(said, menu, welcomeFor("Bob"))
	quoted := regexp.MustCompile(`'([^']+)'`)
	commands := map[string]bool{}
	for _, s := range said {
		for _, m := range quoted.FindAllStringSubmatch(s, -1) {
			commands[strings.ReplaceAll(m[1], "<name>", "goose")] = true
		}
	}
	require.Greater(t, len(commands), 8)

	addr := startGame(t, time.Hour)
	createCharacter(t, addr, "Quillon", "correcthorse")
	c := loginAs(t, addr, "Quillon", "correcthorse")
	for cmd := range commands {
		require.NoError(t, c.Send(cmd))
		text, err := c.Expect(`<\d+/\d+hp`, 5*time.Second)
		require.NoError(t, err, cmd)
		assert.NotContains(t, text, "Unknown request", "an answer tells players to type %q", cmd)
	}
}

// Against the real game: a socialite in Temple Square welcomes a character
// made while it's there, and answers their question.
func TestSocialite_welcomesAndAnswers(t *testing.T) {
	addr := startGame(t, time.Hour)
	createCharacter(t, addr, "Mabel", "correcthorse")
	runAdventurer(t, addr, "Mabel", func(a *Adventurer) { a.cfg.Style = Socialite })

	// in the square, ready for whoever comes
	createCharacter(t, addr, "Watcher", "correcthorse")
	w := loginAs(t, addr, "Watcher", "correcthorse")
	require.Eventually(t, func() bool {
		if w.Send("look") != nil {
			return false
		}
		text, err := room(w, "Temple Square")
		return err == nil && strings.Contains(text, "Mabel is here.")
	}, 20*time.Second, 50*time.Millisecond, "Mabel never got to Temple Square")

	// a newcomer, kept connected, so they're there to be answered
	n := dial(t, addr)
	for _, s := range []struct{ want, send string }{
		{`By what name do you wish to be known\? `, "Newt"},
		{`Create them\? \(yn\) `, "y"},
		{`Which lineage\? `, "1"},
		{`Choose a password: `, "correcthorse"},
		{`Again: `, "correcthorse"},
	} {
		_, err := n.Expect(s.want, 10*time.Second)
		require.NoError(t, err, n.Transcript())
		require.NoError(t, n.Send(s.send))
	}
	_, err := n.Expect(regexp.QuoteMeta(`Mabel says, "`+welcomeFor("Newt")+`".`), 20*time.Second)
	require.NoError(t, err, n.Transcript())

	require.NoError(t, n.Send("say where should I hunt?"))
	_, err = n.Expect(regexp.QuoteMeta(`Mabel says, "`+topicFor("hunt").answer+`".`), 20*time.Second)
	require.NoError(t, err, n.Transcript())
}
