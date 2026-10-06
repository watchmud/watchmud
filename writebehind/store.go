// Package writebehind is a player.Store that takes saves off of the world
// goroutine. Save queues a record and returns; one background goroutine
// writes them to the store it wraps around.
package writebehind

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"
	"uuid"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/player"
)

type Store struct {
	inner   player.Store
	mu      sync.Mutex
	pending map[uuid.UUID]*player.Record // newest unwritten record per player; guarded by mu
	written map[uuid.UUID]*player.Record // last record that reached inner; writer goroutine only

	// retry is the first wait before trying again what failed to write,
	// doubling to maxRetry while it keeps failing.
	retry time.Duration

	wake chan struct{} // buffered 1: "there is something to write"
	quit chan struct{} // closed by Close
	done chan struct{} // closed by the writer once it has finished
}

// batchSaver is an inner store that can write many records in one go.
//
// failed lists the index in recs of every record that was not written, and
// err says why; everything not in failed was written. A batch that failed
// outright lists every index. One bad record must not count against the rest
// of the batch, or it gets rewritten on every flush forever.
type batchSaver interface {
	SaveAll(recs []*player.Record) (failed []int, err error)
}

func New(inner player.Store) *Store { return newRetrying(inner, time.Second) }

// newRetrying is New with retry's first wait chosen: tests don't wait seconds.
func newRetrying(inner player.Store, retry time.Duration) *Store {
	s := &Store{
		inner:   inner,
		pending: make(map[uuid.UUID]*player.Record),
		written: make(map[uuid.UUID]*player.Record),
		retry:   retry,
		wake:    make(chan struct{}, 1), // doorbell pattern
		quit:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	go s.run()
	return s
}

// maxRetry is the longest wait between tries at a store that keeps failing.
const maxRetry = 30 * time.Second

// closeTries is how many more times Close's last flush is tried.
const closeTries = 5

// run writes whenever there's something new, and tries again on its own
// whatever failed: the last player out quitting during a database blip has
// nobody after them whose save would ring the bell. Closing, it keeps
// trying until everything is written, or Close stops waiting.
func (s *Store) run() {
	defer close(s.done)
	wait := s.retry
	var again <-chan time.Time // nil: nothing to retry
	for {
		select {
		case <-s.wake:
		case <-again:
		case <-s.quit:
			// a few more tries, a second apart at most: a database that's
			// briefly away gets its records; one refusing a record for good
			// doesn't hold the shutdown up
			for tries := 0; s.flush() && tries < closeTries; tries++ {
				time.Sleep(min(wait, time.Second))
				wait *= 2
			}
			return
		}
		if s.flush() {
			again = time.After(wait)
			wait = min(wait*2, maxRetry)
		} else {
			again, wait = nil, s.retry
		}
	}
}

// flush writes what's pending, and answers whether anything failed to write.
func (s *Store) flush() (failedAny bool) {
	// copy queue under the lock
	s.mu.Lock()
	batch := make([]*player.Record, 0, len(s.pending))
	for _, r := range s.pending {
		batch = append(batch, r)
	}
	s.mu.Unlock()
	// now do the slow part without the lock:
	// copy the changed entries over to a new slice
	changed := make([]*player.Record, 0, len(batch))
	for _, r := range batch {
		if prev, ok := s.written[r.Id]; ok && reflect.DeepEqual(prev, r) {
			s.forget(r) // nothing changed since last write
			continue
		}
		changed = append(changed, r)
	}
	if bs, ok := s.inner.(batchSaver); ok {
		failed, err := bs.SaveAll(changed)
		unwritten := make(map[int]bool, len(failed))
		for _, i := range failed {
			unwritten[i] = true
		}
		if err != nil {
			log.Error().Err(err).Int("failed", len(failed)).Int("records", len(changed)).Msg("writebehind: batch save failed")
		}
		for i, r := range changed {
			if unwritten[i] {
				// Stays pending, to try again. And whatever the store holds
				// for them is unknown now -- the write may have landed and
				// said otherwise -- so the next record is written whatever
				// it looks like: compared to the last known one, a change
				// undone would be skipped, leaving the store the change.
				delete(s.written, r.Id)
				failedAny = true
				continue
			}
			s.written[r.Id] = r
			s.forget(r)
		}
		return failedAny
	}
	// save one at a time
	for _, r := range changed {
		if err := s.inner.Save(r); err != nil {
			// left in pending, to try again; unknown in the store, as above
			log.Error().Err(err).Str("player", r.Name).Msg("writebehind: save failed")
			delete(s.written, r.Id)
			failedAny = true
			continue
		}
		s.written[r.Id] = r
		s.forget(r)
	}
	return failedAny
}

// Forget takes r out of the queue, unless a newer record replaced it
// while r was being written, in which case that one still needs writing.
func (s *Store) forget(r *player.Record) {
	s.mu.Lock()
	if s.pending[r.Id] == r {
		delete(s.pending, r.Id)
	}
	s.mu.Unlock()
}

func (s *Store) Save(r *player.Record) error {
	if r == nil {
		return nil
	}
	s.mu.Lock()
	s.pending[r.Id] = r
	s.mu.Unlock()
	// if doorbell is quiet, send goes into the buffer.
	// if it's already rung, default branch skips the send instead of blocking.
	select {
	case s.wake <- struct{}{}:
	default: // bell rung; writer will see this too
	}
	return nil
}

// Load answers from the queue before the inner store: a record still
// waiting to be written is newer than anything in the inner store.
func (s *Store) Load(name string) (*player.Record, bool, error) {
	s.mu.Lock()
	for _, r := range s.pending {
		if r.Name == name {
			s.mu.Unlock()
			return r, true, nil
		}
	}
	s.mu.Unlock()
	return s.inner.Load(name)
}

func (s *Store) Close(ctx context.Context) error {
	close(s.quit)
	select {
	case <-s.done:
	case <-ctx.Done():
		return fmt.Errorf("writebehind: gave up waiting for the last writes: %w", ctx.Err())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := len(s.pending); n > 0 {
		return fmt.Errorf("writebehind: %d records were never written", n)
	}
	return nil
}
