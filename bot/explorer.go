package bot

import "context"

// explore walks the world to map it: an exit not yet taken from here if
// there is one, else the shortest known way to the nearest room that has one.
// Like a wanderer it never takes a keepOut door or a closed one, fights only
// what attacks it, and loots nothing. When nothing is left untaken it has seen
// all it may, and wanders. The atlas outlives deaths and recalls -- a recall
// lands somewhere already on it -- but not a reconnect.
func (a *Adventurer) explore(ctx context.Context) (stateFn, error) {
	_, m, err := a.ask("look", roomBlockRe)
	if err != nil {
		return nil, err
	}
	a.here = m[1]
	a.atlas.see(a.here, parseExits(m[2]))
	for range wanderSteps {
		if next, err := a.mend(ctx); next != nil || err != nil {
			return next, err
		}
		dir := a.nextUntaken()
		if dir == "" {
			a.log("mapped %d rooms; wandering", a.atlas.size())
			return a.wander, nil
		}
		if err := a.pause(ctx, a.cfg.Pace.Walk); err != nil {
			return nil, err
		}
		from := a.here
		_, m, err := a.ask(dir, roomBlockRe)
		if err != nil {
			return nil, err
		}
		a.here = m[1]
		a.atlas.learn(from, dir, a.here)
		a.atlas.see(a.here, parseExits(m[2]))
		a.count(func(st *Stats) { st.Steps++; st.Mapped = a.atlas.size() })
		a.say(momentWander)
	}
	return a.explore, nil
}

// nextUntaken is the next step towards an exit not yet taken -- one from here,
// chosen at random, or the first step of the way to the nearest room with
// one -- or "" when there's none left it may take.
func (a *Adventurer) nextUntaken() string {
	if here := a.atlas.untaken(a.here); len(here) > 0 {
		return here[a.rng.IntN(len(here))]
	}
	path := a.atlas.route(a.here, func(room string) bool { return len(a.atlas.untaken(room)) > 0 })
	if len(path) == 0 {
		return ""
	}
	return path[0]
}
