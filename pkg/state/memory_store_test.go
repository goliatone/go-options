package state_test

import (
	"context"
	"fmt"

	"github.com/goliatone/go-options/pkg/state"
)

// memoryStore is a minimal in-memory Store used by contract tests/examples.
// It intentionally makes no persistence assumptions beyond deterministic keys.
type memoryStore[T any] struct {
	records map[string]memoryRecord[T]
}

type memoryRecord[T any] struct {
	snapshot T
	meta     state.Meta
}

func newMemoryStore[T any]() *memoryStore[T] {
	return &memoryStore[T]{records: map[string]memoryRecord[T]{}}
}

func (s *memoryStore[T]) put(key string, snapshot T, meta state.Meta) {
	s.records[key] = memoryRecord[T]{snapshot: snapshot, meta: meta}
}

func (s *memoryStore[T]) Load(_ context.Context, ref state.Ref) (T, state.Meta, bool, error) {
	var zero T
	key := memoryStoreKey(ref)
	record, ok := s.records[key]
	if !ok {
		return zero, state.Meta{}, false, nil
	}
	return record.snapshot, record.meta, true, nil
}

func (s *memoryStore[T]) Save(_ context.Context, ref state.Ref, snapshot T, meta state.Meta) (state.Meta, error) {
	key := memoryStoreKey(ref)
	s.records[key] = memoryRecord[T]{snapshot: snapshot, meta: meta}
	return meta, nil
}

func memoryStoreKey(ref state.Ref) string {
	return fmt.Sprintf("%s|%s|%s", ref.Domain, ref.Scope.Name, scopeID(ref))
}

func scopeID(ref state.Ref) string {
	switch ref.Scope.Name {
	case "user":
		return stringValue(ref.Scope.Metadata["user_id"])
	case "team":
		return stringValue(ref.Scope.Metadata["team_id"])
	case "org":
		return stringValue(ref.Scope.Metadata["org_id"])
	case "tenant":
		return stringValue(ref.Scope.Metadata["tenant_id"])
	case "system":
		return ""
	default:
		return stringValue(ref.Scope.Metadata["id"])
	}
}

func stringValue(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
