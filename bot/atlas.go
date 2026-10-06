package bot

// atlas is the world as an explorer has seen it: rooms by name -- names are
// unique, TestRoomNames_uniqueInTheWorld sees to it -- and each one's open
// exits, by the direction typed, to the room they lead to, or "" for one not
// taken yet. A closed door isn't on it: the explorer doesn't open doors.
type atlas struct {
	rooms map[string]map[string]string
}

func newAtlas() *atlas { return &atlas{rooms: map[string]map[string]string{}} }

// see records a room and its exits, keeping whatever is already known of them.
func (m *atlas) see(room string, exits []string) {
	known := m.rooms[room]
	if known == nil {
		known = map[string]string{}
		m.rooms[room] = known
	}
	for _, dir := range exits {
		if _, ok := known[dir]; !ok {
			known[dir] = ""
		}
	}
}

// learn is a step taken: dir from from arrived in to.
func (m *atlas) learn(from, dir, to string) {
	if m.rooms[from] == nil {
		m.rooms[from] = map[string]string{}
	}
	m.rooms[from][dir] = to
}

// untaken is room's exits not yet walked, that an explorer may take, in a
// stable order.
func (m *atlas) untaken(room string) []string {
	var out []string
	for _, dir := range directionOrder {
		if to, ok := m.rooms[room][dir]; ok && to == "" && !kept(room, dir) {
			out = append(out, dir)
		}
	}
	return out
}

// route is the shortest walk over known exits from from to the nearest room
// want says yes to, as the directions to type, or nil if none can be reached.
// It never routes through a keepOut door. from itself counts: a route of no
// steps is an empty, non-nil slice.
func (m *atlas) route(from string, want func(room string) bool) []string {
	type step struct {
		room string
		path []string
	}
	seen := map[string]bool{from: true}
	queue := []step{{from, []string{}}}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		if want(s.room) {
			return s.path
		}
		for _, dir := range directionOrder {
			to := m.rooms[s.room][dir]
			if to == "" || seen[to] || kept(s.room, dir) {
				continue
			}
			seen[to] = true
			queue = append(queue, step{to, append(append([]string{}, s.path...), dir)})
		}
	}
	return nil
}

// size is how many rooms it has seen.
func (m *atlas) size() int { return len(m.rooms) }

// directionOrder is every direction, so routes and choices don't depend on
// map order.
var directionOrder = []string{"north", "east", "south", "west", "up", "down"}
