// Package store is a thin bbolt wrapper providing durable key/value buckets
// at /etc/travo/travo.db. Chosen over SQLite (modernc.org/sqlite adds ~10 MB
// to a 12 MB binary) and flat JSON (no transactional multi-key writes) — see
// docs/plans/2026-07-08-hardening-followups-and-persistence.md.
//
// Flash-write discipline: callers must batch writes (periodic flushes, not
// per-request/per-sample) — /etc/travo sits on NAND-backed overlayfs.
package store

import (
	"time"

	bolt "go.etcd.io/bbolt"
)

// Store wraps a bbolt database with string-keyed bucket helpers.
type Store struct {
	db *bolt.DB
}

// Open opens (or creates) the database at path with 0600 permissions.
// The open times out instead of blocking forever on a stale file lock.
func Open(path string) (*Store, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases the database file lock.
func (s *Store) Close() error {
	return s.db.Close()
}

// Put stores value under bucket/key, creating the bucket if needed.
func (s *Store) Put(bucket, key string, value []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucket))
		if err != nil {
			return err
		}
		return b.Put([]byte(key), value)
	})
}

// Get returns the value under bucket/key, or nil when the bucket or key is missing.
func (s *Store) Get(bucket, key string) ([]byte, error) {
	var out []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		if v := b.Get([]byte(key)); v != nil {
			out = make([]byte, len(v))
			copy(out, v)
		}
		return nil
	})
	return out, err
}

// Delete removes bucket/key; missing bucket or key is not an error.
func (s *Store) Delete(bucket, key string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		return b.Delete([]byte(key))
	})
}

// ForEach iterates all key/value pairs in bucket; a missing bucket iterates nothing.
func (s *Store) ForEach(bucket string, fn func(k, v []byte) error) error {
	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		return b.ForEach(fn)
	})
}
