package state

import (
	"context"
	"sync"
)

// MemoryStore is a minimal in-memory Store implementation intended for tests
// and examples. It uses Ref.Identifier() as its deterministic key and makes no
// persistence assumptions beyond that.
type MemoryStore[T any] struct {
	mu      sync.RWMutex
	records map[string]memoryRecord[T]
}

type memoryRecord[T any] struct {
	snapshot T
	meta     Meta
}

func NewMemoryStore[T any]() *MemoryStore[T] {
	return &MemoryStore[T]{records: map[string]memoryRecord[T]{}}
}

func (s *MemoryStore[T]) Load(_ context.Context, ref Ref) (T, Meta, bool, error) {
	var zero T
	key, err := ref.Identifier()
	if err != nil {
		return zero, Meta{}, false, err
	}

	s.mu.RLock()
	record, ok := s.records[key]
	s.mu.RUnlock()
	if !ok {
		return zero, Meta{}, false, nil
	}
	return record.snapshot, cloneMeta(record.meta), true, nil
}

func (s *MemoryStore[T]) Save(_ context.Context, ref Ref, snapshot T, meta Meta) (Meta, error) {
	key, err := ref.Identifier()
	if err != nil {
		return Meta{}, err
	}

	s.mu.Lock()
	s.records[key] = memoryRecord[T]{snapshot: snapshot, meta: cloneMeta(meta)}
	s.mu.Unlock()
	return cloneMeta(meta), nil
}

func cloneMeta(meta Meta) Meta {
	out := meta
	if meta.Extra == nil {
		return out
	}
	out.Extra = make(map[string]string, len(meta.Extra))
	for k, v := range meta.Extra {
		out.Extra[k] = v
	}
	return out
}

