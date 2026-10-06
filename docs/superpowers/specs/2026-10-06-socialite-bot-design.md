# The Socialite

Status: design, 2026-10-06. ROADMAP "Inhabitants": "a **Socialite** (greets new
characters in town, answers newbie questions)".

## Goal

A new player's first minute has somebody in it. Every new character lands in Temple
Square; a socialite stands there, says welcome, and answers the questions a newcomer
has -- where to fight, how to get home, how to heal, where to buy and repair -- without
pretending to be a person.

## Behaviour

- **Stays in Temple Square.** Home is where it goes after login, a fight, a death or
  getting lost. It never walks off.
- **Welcomes new characters, once each.** The server tells the room a character is
  new: `event.EnteredGame.First`, from `World.ArriveNew` at creation, rendered "X has
  entered the game for the first time." Returning players get nothing; the bot says
  who it is in the welcome ("I'm a bot who answers questions").
- **Answers questions** from a fixed table (`bot/faq.go`): topics of keywords, in
  order, first match wins; the menu of topics when none matches.
  - Said in the room, it answers only what is put to it: something naming it,
    starting with "help", or a question ("?") it has a topic for. Anything else is
    somebody else's conversation.
  - Told, it always answers, by tell: a topic, or the menu.
  - One answer per person per 20 seconds, never to another bot, after a typing pause.
- Otherwise a bot like the others: it fights back, flees, recalls, and wakes in the
  square when it dies.

## Truth

The answers are documentation for the newest players, so they have to stay true.
`TestFAQ_commandsTheGameKnows` sends every quoted command to the real game and fails
on an unknown one. The facts (where the smithy is, what recall costs) are checked by
hand when the world changes; `help` and the site guide are the other two places.

## Not now

Answers about where a named mob or item is; anything learned from play; more than
one language of keywords; greeting returning players.
