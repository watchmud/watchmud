# site/ for the next release (draft)

`site/` publishes the moment it reaches master, so what's below waits for the release
that ships PR #36. Rows are for the "Commands worth knowing" table in
`site/index.html`, beside the ones named. Delete this file once they're in.

**Already wrong on the live site:** the `look` row says "`look goose` for one thing
in it", and in v0.10.0 that showed the room whatever you named. It works on master as
of 2026-10-06, so the row becomes true with this release -- no edit needed, but don't
release without it.

After the `get all from corpse` row:

```html
<tr><td><code>put pelt in satchel</code></td><td>Put something away in a bag you carry, or in an open chest. <code>get pelt from satchel</code> takes it back out; <code>look in satchel</code> shows what's inside</td></tr>
<tr><td><code>give pelt to Ana</code></td><td>Hand something to a player in the same room. <code>give 20 coins to Ana</code> for coins</td></tr>
```

After the `drop knife` row:

```html
<tr><td><code>donate knife</code>, <code>junk knife</code></td><td>Done with something? Donate sends it, from anywhere, to the donation room for someone who needs it. Junk destroys it: nothing back, just the space</td></tr>
```

After the `remove` row:

```html
<tr><td><code>open grate</code>, <code>close grate</code></td><td>A door, by its name or its direction (<code>open west</code>), or a chest. A closed door shows as "(closed)" among the exits</td></tr>
<tr><td><code>unlock grate</code>, <code>lock grate</code></td><td>With its key, if you're carrying it (a key in a bag counts)</td></tr>
```

The `look` row, extended:

```html
<tr><td><code>look</code></td><td>Describe the room again (<code>l</code> for short), or <code>look goose</code> for one thing in it: a creature, a player, or an item's power, condition and what it lets you cast. <code>examine</code> works too</td></tr>
```

After the `list` row's neighbours, nothing: the satchel is just on the General Store's
list.

After the `tell` row:

```html
<tr><td><code>ooc hello</code></td><td>Chat with everyone playing who's on the channel: questions, plans, anything out of the game. <code>ooc off</code> leaves it. The Temple Square bot answers questions there too</td></tr>
<tr><td><code>follow Ana</code></td><td>Walk wherever Ana walks, as her group. <code>follow</code> on its own stops</td></tr>
<tr><td><code>group</code>, <code>gt hello</code></td><td>How your group is doing and where; <code>gt</code> talks to just them</td></tr>
```

"Things that surprise people" could gain one:

```html
<dt>My coins won't go in my bag</dt>
<dd>Coins stay in your purse. Bags hold things, ten of them for the leather satchel, and nothing that holds things goes inside another.</dd>
```

And one for the janitor:

```html
<dt>Something I dropped vanished</dt>
<dd>The town janitor sweeps up what's been left lying in Wrathrock for five minutes or more. Give it to someone instead, or leave it in the donation room, east of Temple Square, which he never touches. Out in the Hollowfields it may have been a crow: catch it, and whatever it took is in its corpse.</dd>
```
