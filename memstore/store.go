package memstore

import (
	"sync"
	"uuid"

	"github.com/watchmud/watchmud/player"
)

type Store struct {
	mu     sync.Mutex
	byId   map[uuid.UUID]*player.Record
	byName map[string]*player.Record
}

func New() *Store {
	return &Store{
		byId:   make(map[uuid.UUID]*player.Record),
		byName: make(map[string]*player.Record),
	}
}

func (s *Store) Load(name string) (*player.Record, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, found := s.byName[name]
	return r, found, nil
}

func (s *Store) Save(r *player.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byName[r.Name] = r
	s.byId[r.Id] = r
	return nil
}
