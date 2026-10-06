package mongostore

import (
	"context"
	"os"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/report"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// These need a real mongo, which `make db-up` starts and `make test-db` points
// at. They skip rather than fail without one, so `make check` stays runnable
// on a machine with no docker -- the document mapping, which is where the bugs
// actually are, is covered by document_test.go with no database at all.
const uriEnv = "WATCHMUD_TEST_MONGO_URI"

// newTestStore connects to a database of its own, dropped when the test ends,
// so a run never sees another run's characters.
func newTestStore(t *testing.T) *Store {
	t.Helper()

	uri := os.Getenv(uriEnv)
	if uri == "" {
		t.Skipf("set %s to run this (see: make db-up, make test-db)", uriEnv)
	}

	ctx := context.Background()
	dbName := "watchmud_test_" + uuid.New().String()[:8]

	s, err := New(ctx, uri, dbName)
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, s.players.Database().Drop(context.Background()))
		assert.NoError(t, s.Close(context.Background()))
	})
	return s
}

func TestStore_saveThenLoad(t *testing.T) {
	s := newTestStore(t)
	rec := testRecord()

	require.NoError(t, s.Save(rec))

	got, found, err := s.Load(rec.Name)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, rec, got)
}

func TestStore_loadUnknownPlayer(t *testing.T) {
	s := newTestStore(t)

	got, found, err := s.Load("nobody")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, got)
}

// Saving the same character twice replaces the one document rather than
// growing a second: the world saves after every command.
func TestStore_saveIsAnUpsert(t *testing.T) {
	s := newTestStore(t)
	rec := testRecord()

	require.NoError(t, s.Save(rec))
	rec.CurHealth = 12
	rec.Inventory = rec.Inventory[:1]
	require.NoError(t, s.Save(rec))

	count, err := s.players.CountDocuments(context.Background(), bson.D{})
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)

	got, found, err := s.Load(rec.Name)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 12, got.CurHealth)
	assert.Len(t, got.Inventory, 1)
}

// the document is written the way a person reading the collection would want
// it: string ids and a timestamp, not sixteen-byte arrays.
func TestStore_documentShape(t *testing.T) {
	s := newTestStore(t)
	rec := testRecord()
	require.NoError(t, s.Save(rec))

	var raw bson.M
	require.NoError(t, s.players.FindOne(context.Background(), bson.D{{Key: "name", Value: rec.Name}}).Decode(&raw))

	assert.Equal(t, rec.Id.String(), raw["_id"])
	assert.Equal(t, rec.Name, raw["name"])
	assert.Equal(t, "hill_dwarf", raw["lineage_id"])
	assert.Len(t, raw["inventory"], len(rec.Inventory))
	assert.Len(t, raw["equipment"], 1)
	assert.WithinDuration(t, time.Now(), raw["updated_at"].(bson.DateTime).Time(), time.Minute)

	// and a bag's contents come back inside it
	got, found, err := s.Load(rec.Name)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, rec.Inventory, got.Inventory)
}

// Two characters with one name is the one thing the game can't sort out for
// itself: nothing in handleCreatePlayer checks, so the index has to.
func TestStore_nameIsUnique(t *testing.T) {
	s := newTestStore(t)

	first := testRecord()
	require.NoError(t, s.Save(first))

	impostor := testRecord() // same name, different id
	assert.ErrorContains(t, s.Save(impostor), "duplicate key")
}

// a character who saved with nothing loads with nothing, rather than failing
// to decode a missing array.
func TestStore_emptyGear(t *testing.T) {
	s := newTestStore(t)
	rec := &player.Record{
		Id:        uuid.New(),
		Name:      "naked",
		CurHealth: 100,
		MaxHealth: 100,
		LineageId: "human",
	}
	require.NoError(t, s.Save(rec))

	got, found, err := s.Load("naked")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, rec, got)
}

func TestStore_saveAll(t *testing.T) {
	s := newTestStore(t)
	a, b := testRecord(), testRecord()
	a.Id, a.Name = uuid.New(), "alpha"
	b.Id, b.Name = uuid.New(), "beta"

	failed, err := s.SaveAll([]*player.Record{a, b})
	require.NoError(t, err)
	assert.Empty(t, failed)

	for _, name := range []string{"alpha", "beta"} {
		_, found, err := s.Load(name)
		require.NoError(t, err)
		assert.True(t, found, name)
	}
}

// A document the server refuses -- here, a second character with a name
// already taken -- is reported by its index, and the rest of the batch is
// written anyway.
func TestStore_saveAllReportsWhichFailed(t *testing.T) {
	s := newTestStore(t)
	first := testRecord()
	first.Id, first.Name = uuid.New(), "taken"
	require.NoError(t, s.Save(first))

	dup, other := testRecord(), testRecord()
	dup.Id, dup.Name = uuid.New(), "taken"
	other.Id, other.Name = uuid.New(), "other"

	failed, err := s.SaveAll([]*player.Record{dup, other})
	require.Error(t, err)
	assert.Equal(t, []int{0}, failed)

	_, found, err := s.Load("other")
	require.NoError(t, err)
	assert.True(t, found, "the good one was written")
}

// a report is one document in its own collection
func TestStore_fileReport(t *testing.T) {
	s := newTestStore(t)
	r := report.Report{Kind: "bug", Player: "Ann", Room: "wrathrock/market_square", Text: "the fountain hums", At: time.Now().UTC().Truncate(time.Millisecond)}
	require.NoError(t, s.File(r))

	var got reportDoc
	require.NoError(t, s.reports.FindOne(context.Background(), bson.D{{Key: "player", Value: "Ann"}}).Decode(&got))
	assert.Equal(t, reportDoc(r), got)
}
