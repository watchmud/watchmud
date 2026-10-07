// Package mongostore is the player.Store that survives a restart: one
// document per character in a mongo collection.
//
// The shape is deliberately boring. Load is a find by name, Save is an upsert
// by id, and there is no caching, no batching and no write queue. See the note
// on Save about what that costs and when to care.
package mongostore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/watchmud/watchmud/player"
)

// DefaultDatabase is used when the config names a uri but no database.
const DefaultDatabase = "watchmud"

// CollectionName is where characters live. Exported so a test or a script can
// go looking in the same place the server writes.
const CollectionName = "players"

// ReportsName is where bug, idea and typo reports go: one document each.
const ReportsName = "reports"

// defaultTimeout bounds one operation. player.Store has no context to pass --
// its callers are the world goroutine and the login path, neither of which has
// one -- so the deadline is set here. It exists so an unreachable mongo fails
// the command instead of parking the single goroutine that runs the whole MUD.
const defaultTimeout = 5 * time.Second

type Store struct {
	client  *mongo.Client
	players *mongo.Collection
	reports *mongo.Collection
	timeout time.Duration
}

// New connects, verifies the deployment is actually reachable, and makes sure
// the name index exists.
//
// The ping is the point: mongo.Connect only validates the options, so without
// it a typo in the uri turns into a server that starts happily and then fails
// every save. Startup is where you want to find that out.
func New(ctx context.Context, uri string, database string) (*Store, error) {
	if uri == "" {
		return nil, errors.New("mongostore: no uri configured")
	}
	if database == "" {
		database = DefaultDatabase
	}

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongostore: connect: %w", err)
	}

	s := &Store{
		client:  client,
		players: client.Database(database).Collection(CollectionName),
		reports: client.Database(database).Collection(ReportsName),
		timeout: defaultTimeout,
	}

	pingCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		// don't leak the monitoring goroutines of a client nobody will use
		_ = client.Disconnect(context.WithoutCancel(pingCtx))
		return nil, fmt.Errorf("mongostore: ping %s: %w", database, err)
	}

	if err := s.ensureIndexes(ctx); err != nil {
		_ = client.Disconnect(context.WithoutCancel(ctx))
		return nil, err
	}
	return s, nil
}

// ensureIndexes makes the login lookup an index hit and, more importantly,
// makes two characters with one name impossible. Nothing in handleCreatePlayer
// checks for a name that is already taken -- the database is the only thing
// that can say no without a race.
func (s *Store) ensureIndexes(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	_, err := s.players.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("name_unique"),
	})
	if err != nil {
		return fmt.Errorf("mongostore: creating name index: %w", err)
	}
	return nil
}

// Close disconnects the client. Safe on a nil store so a caller can defer it
// next to a New that might have failed.
func (s *Store) Close(ctx context.Context) error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Disconnect(ctx)
}

// Load one character by name. Not found is (nil, false, nil): a new player
// typing a name nobody has used is the create-a-character path, not an error.
func (s *Store) Load(name string) (*player.Record, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	var doc playerDoc
	err := s.players.FindOne(ctx, bson.D{{Key: "name", Value: name}}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("mongostore: load %s: %w", name, err)
	}

	rec, err := doc.record()
	if err != nil {
		return nil, false, fmt.Errorf("mongostore: load %s: %w", name, err)
	}
	return rec, true, nil
}

// Save writes the whole character as one document, replacing whatever was
// there. Upsert, so creation and every save after it are the same call.
//
// The record is a snapshot the caller already built, so what lands is a
// consistent picture of one character and not a half-applied command. What it
// is not is cheap: world.HandleIncomingMessage saves after every single
// command, on the one goroutine that runs the game, so every `look` is now a
// round trip the whole MUD waits on. That is fine against a mongo on
// localhost and it is the reason Store is an interface -- batching or
// debouncing belongs in here, where world/ stays unaware of it. See the TODO
// on world.save.
func (s *Store) Save(r *player.Record) error {
	if r == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	doc := newPlayerDoc(r, time.Now())
	_, err := s.players.ReplaceOne(ctx,
		bson.D{{Key: "_id", Value: doc.Id}},
		doc,
		options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("mongostore: save %s: %w", r.Name, err)
	}
	return nil
}

// SaveAll writes many characters in one round trip. Unordered, so one
// bad document doesn't stop the rest.
//
// failed is the index in recs of every record that was not written: the ones
// the server rejected, or all of them if the batch never got that far.
func (s *Store) SaveAll(recs []*player.Record) ([]int, error) {
	if len(recs) == 0 {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	now := time.Now()
	models := make([]mongo.WriteModel, 0, len(recs))
	for _, r := range recs {
		doc := newPlayerDoc(r, now)
		models = append(models, mongo.NewReplaceOneModel().
			SetFilter(bson.D{{Key: "_id", Value: doc.Id}}).
			SetReplacement(doc).
			SetUpsert(true))
	}
	_, err := s.players.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	if err == nil {
		return nil, nil
	}
	err = fmt.Errorf("mongostore: save %d players: %w", len(recs), err)

	// Unordered, so the server carried on past each failure and says which
	// ones they were -- unless the write concern failed too, in which case
	// nothing is known to have landed. Rewriting a record that did land is
	// harmless (it is a whole-document upsert), so when in doubt, all of them.
	var bwe mongo.BulkWriteException
	if errors.As(err, &bwe) && bwe.WriteConcernError == nil && len(bwe.WriteErrors) > 0 {
		failed := make([]int, 0, len(bwe.WriteErrors))
		for _, we := range bwe.WriteErrors {
			failed = append(failed, we.Index)
		}
		return failed, err
	}
	failed := make([]int, len(recs))
	for i := range failed {
		failed[i] = i
	}
	return failed, err
}
