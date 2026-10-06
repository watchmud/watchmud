package writebehind

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/memstore"
	"github.com/watchmud/watchmud/player"
)

// gatedStore holds every write until the test lets it through, so a test can
// catch the writer mid-save. Writes are counted by the writer goroutine and
// read only after Close, which is what makes that safe.
type gatedStore struct {
	*memstore.Store
	release chan struct{}
	saves   int
}

func newGated() *gatedStore {
	return &gatedStore{
		Store:   memstore.New(),
		release: make(chan struct{}),
	}
}

func (g *gatedStore) Save(r *player.Record) error {
	<-g.release
	g.saves++
	return g.Store.Save(r)
}

func TestSaveReachesTheInnerStore(t *testing.T) {
	inner := memstore.New()
	s := New(inner)
	require.NoError(t, s.Save(&player.Record{Id: uuid.New(), Name: "dood", CurHealth: 5}))
	require.NoError(t, s.Close(context.Background()))

	got, found, err := inner.Load("dood")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 5, got.CurHealth)
}

// quit and log straight back in: the record is still on its way to the
// inner store, and Load has to find it anyway.
func TestLoadSeesARecordStillBeingWritten(t *testing.T) {
	inner := newGated()
	s := New(inner)
	require.NoError(t, s.Save(&player.Record{Id: uuid.New(), Name: "dood", CurHealth: 5}))

	got, found, err := s.Load("dood") // parked in inner.Save
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 5, got.CurHealth)

	close(inner.release)
	require.NoError(t, s.Close(context.Background()))
}

// Saves that pile up behind a slow write become one write of the newest.
func TestSavesBehindASlowWriteCollapse(t *testing.T) {
	inner := newGated()
	s := New(inner)
	id := uuid.New()

	require.NoError(t, s.Save(&player.Record{Id: id, Name: "dood", CurHealth: 1}))
	for hp := 2; hp <= 10; hp++ {
		require.NoError(t, s.Save(&player.Record{Id: id, Name: "dood", CurHealth: hp}))
	}
	close(inner.release)
	require.NoError(t, s.Close(context.Background()))

	got, _, _ := inner.Load("dood")
	assert.Equal(t, 10, got.CurHealth, "newest one wins")
	assert.LessOrEqual(t, inner.saves, 2, "not ten writes")
}

// batchStore takes records in batches and refuses the ones named in reject,
// the way mongo's unordered bulk write carries on past a bad document.
type batchStore struct {
	*memstore.Store
	reject  map[string]bool
	batches int
}

func (b *batchStore) SaveAll(recs []*player.Record) ([]int, error) {
	b.batches++
	var failed []int
	for i, r := range recs {
		if b.reject[r.Name] {
			failed = append(failed, i)
			continue
		}
		_ = b.Store.Save(r)
	}
	if len(failed) > 0 {
		return failed, errors.New("some were refused")
	}
	return nil, nil
}

// One record the store refuses doesn't hold the rest of the batch hostage:
// they are written and leave the queue, and only the bad one is left.
func TestBatch_oneBadRecordOnlyFailsItself(t *testing.T) {
	inner := &batchStore{Store: memstore.New(), reject: map[string]bool{"bad": true}}
	s := newRetrying(inner, time.Millisecond)

	require.NoError(t, s.Save(&player.Record{Id: uuid.New(), Name: "good"}))
	require.NoError(t, s.Save(&player.Record{Id: uuid.New(), Name: "bad"}))
	require.NoError(t, s.Save(&player.Record{Id: uuid.New(), Name: "fine"}))

	err := s.Close(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 records were never written")

	for _, name := range []string{"good", "fine"} {
		_, found, _ := inner.Store.Load(name)
		assert.True(t, found, name)
	}
	_, found, _ := inner.Store.Load("bad")
	assert.False(t, found)
}

// flakyStore fails its first n writes, then works -- a database blip.
type flakyStore struct {
	*memstore.Store
	mu    sync.Mutex
	fails int
}

func (f *flakyStore) Save(r *player.Record) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fails > 0 {
		f.fails--
		return errors.New("blip")
	}
	return f.Store.Save(r)
}

func (f *flakyStore) Load(name string) (*player.Record, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Store.Load(name)
}

// A save that fails is tried again on its own: the last player out
// quitting during a blip has nobody after them to ring the bell.
func TestRetriesWithoutAnotherSave(t *testing.T) {
	inner := &flakyStore{Store: memstore.New(), fails: 2}
	s := newRetrying(inner, time.Millisecond)
	require.NoError(t, s.Save(&player.Record{Id: uuid.New(), Name: "last"}))

	assert.Eventually(t, func() bool {
		_, found, _ := inner.Load("last")
		return found
	}, 2*time.Second, 5*time.Millisecond)
	require.NoError(t, s.Close(context.Background()))
}

// unsureStore says a write failed, having written it -- a timeout after the
// database had done it -- while lie is set.
type unsureStore struct {
	flakyStore
	lie bool
}

func (u *unsureStore) Save(r *player.Record) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	_ = u.Store.Save(r)
	if u.lie {
		u.lie = false
		return errors.New("timed out, maybe")
	}
	return nil
}

func (u *unsureStore) lieOnce() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.lie = true
}

// After a write it can't be sure of, the next record is written even if it
// matches the last one known to be: a change undone in between (remove a
// sword, wield it again) mustn't leave the store with the change.
func TestAnUnsureWriteIsNotTrusted(t *testing.T) {
	inner := &unsureStore{flakyStore: flakyStore{Store: memstore.New()}}
	s := newRetrying(inner, time.Hour) // the test does its own retrying, through Save
	id := uuid.New()
	wielding := &player.Record{Id: id, Name: "dood", CurHealth: 1}

	require.NoError(t, s.Save(wielding))
	require.Eventually(t, func() bool { _, ok, _ := inner.Load("dood"); return ok }, time.Second, time.Millisecond)

	inner.lieOnce()
	require.NoError(t, s.Save(&player.Record{Id: id, Name: "dood", CurHealth: 2})) // lands, "fails"
	require.Eventually(t, func() bool { r, _, _ := inner.Load("dood"); return r.CurHealth == 2 }, time.Second, time.Millisecond)

	again := *wielding
	require.NoError(t, s.Save(&again))
	require.NoError(t, s.Close(context.Background()))
	got, _, _ := inner.Store.Load("dood")
	assert.Equal(t, 1, got.CurHealth, "the store has what the player has")
}
