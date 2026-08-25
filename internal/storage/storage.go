package storage

import (
	"cityflood/internal/domain"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Store struct {
	mu       sync.Mutex
	dir      string
	state    State
	seq      int64
	prevHash string
}

func New(dir string) (*Store, error) {
	s := &Store{dir: dir}
	s.state = State{Facilities: map[string]*domain.Facility{}, Batches: map[string]*domain.Batch{}, Defects: map[string]*domain.Defect{}, Permits: map[string]*domain.Permit{}, Idempotency: map[string]json.RawMessage{}, IdempotencyMeta: map[string]IdempotencyRecord{}}
	if dir == "" {
		return s, nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return s, nil
}
func (s *Store) Snapshot() State { s.mu.Lock(); defer s.mu.Unlock(); return s.state }
func (s *Store) Commit(typ, agg, actor string, data map[string]any) error {
	_, err := s.CommitEvent(typ, agg, actor, data)
	return err
}
func (s *Store) CommitEvent(typ, agg, actor string, data map[string]any) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	e := domain.AuditEvent{ID: fmt.Sprintf("evt-%d", s.seq), Type: typ, AggregateID: agg, At: time.Now().UTC(), Actor: actor, Data: data, Sequence: s.seq, PrevHash: s.prevHash, SchemaVersion: 1}
	raw, _ := json.Marshal(e)
	sum := sha256.Sum256(raw)
	e.Hash = fmt.Sprintf("%x", sum[:])
	s.prevHash = e.Hash
	s.state.Events = append(s.state.Events, e)
	if err := s.persistLocked(); err != nil {
		return "", err
	}
	return e.ID, nil
}
func (s *Store) Persist() error { s.mu.Lock(); defer s.mu.Unlock(); return s.persistLocked() }
func (s *Store) persistLocked() error {
	if s.dir == "" {
		return nil
	}
	logTmp := filepath.Join(s.dir, "events.tmp")
	lf, err := os.Create(logTmp)
	if err != nil {
		return err
	}
	for _, event := range s.state.Events {
		raw, _ := json.Marshal(event)
		if _, err = lf.Write(append(raw, '\n')); err != nil {
			lf.Close()
			return err
		}
	}
	if err = lf.Sync(); err != nil {
		lf.Close()
		return err
	}
	if err = lf.Close(); err != nil {
		return err
	}
	if err = os.Rename(logTmp, filepath.Join(s.dir, "events.jsonl")); err != nil {
		return err
	}
	tmp := filepath.Join(s.dir, "snapshot.tmp")
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	if err = enc.Encode(s.state); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.dir, "snapshot.json"))
}
func (s *Store) Load() error {
	if s.dir == "" {
		return nil
	}
	p := filepath.Join(s.dir, "snapshot.json")
	cachePath := filepath.Join(s.dir, "snapshot.cache")
	raw, err := os.ReadFile(cachePath)
	if os.IsNotExist(err) {
		raw, err = os.ReadFile(p)
		if err == nil {
			err = os.WriteFile(cachePath, raw, 0644)
		}
	}
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = json.Unmarshal(raw, &s.state); err != nil {
		return err
	}
	for _, e := range s.state.Events {
		if e.Sequence > s.seq {
			s.seq = e.Sequence
		}
		s.prevHash = e.Hash
	}
	if s.state.Facilities == nil {
		s.state.Facilities = map[string]*domain.Facility{}
	}
	if s.state.Batches == nil {
		s.state.Batches = map[string]*domain.Batch{}
	}
	if s.state.Defects == nil {
		s.state.Defects = map[string]*domain.Defect{}
	}
	if s.state.Permits == nil {
		s.state.Permits = map[string]*domain.Permit{}
	}
	if s.state.Idempotency == nil {
		s.state.Idempotency = map[string]json.RawMessage{}
	}
	if s.state.IdempotencyMeta == nil {
		s.state.IdempotencyMeta = map[string]IdempotencyRecord{}
	}
	return nil
}
func (s *Store) GetIdempotent(key string, out any) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, ok := s.state.Idempotency[key]
	if ok {
		_ = json.Unmarshal(raw, out)
	}
	return ok
}
func (s *Store) PutIdempotent(key string, val any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, _ := json.Marshal(val)
	s.state.Idempotency[key] = raw
	_ = s.persistLocked()
}
func (s *Store) GetIdempotencyRecord(key string) (IdempotencyRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.state.IdempotencyMeta[key]
	return r, ok
}
func (s *Store) PutIdempotencyRecord(r IdempotencyRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.IdempotencyMeta == nil {
		s.state.IdempotencyMeta = map[string]IdempotencyRecord{}
	}
	s.state.IdempotencyMeta[r.Key] = r
	return s.persistLocked()
}
func (s *Store) DeleteIdempotencyRecord(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.state.IdempotencyMeta, key)
	return s.persistLocked()
}
func (s *Store) Lock()   { s.mu.Lock() }
func (s *Store) Unlock() { s.mu.Unlock() }
