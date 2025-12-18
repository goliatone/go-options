package state_test

import (
	"context"
	"errors"
	"testing"

	opts "github.com/goliatone/go-options"
	"github.com/goliatone/go-options/pkg/state"
)

type mutateStore[T any] struct {
	loadSnapshot T
	loadMeta     state.Meta
	loadOK       bool
	loadErr      error

	saveCalls   int
	savedRef    state.Ref
	savedMeta   state.Meta
	savedValue  T
	saveReturn  state.Meta
	saveErr     error
}

func (s *mutateStore[T]) Load(_ context.Context, ref state.Ref) (T, state.Meta, bool, error) {
	var zero T
	if s.loadErr != nil {
		return zero, state.Meta{}, false, s.loadErr
	}
	return s.loadSnapshot, s.loadMeta, s.loadOK, nil
}

func (s *mutateStore[T]) Save(_ context.Context, ref state.Ref, snapshot T, meta state.Meta) (state.Meta, error) {
	s.saveCalls++
	s.savedRef = ref
	s.savedMeta = meta
	s.savedValue = snapshot
	if s.saveErr != nil {
		return state.Meta{}, s.saveErr
	}
	return s.saveReturn, nil
}

type validatingConfig struct {
	Name string
}

func (c validatingConfig) Validate() error {
	if c.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func TestResolverMutateValidationFailureDoesNotSave(t *testing.T) {
	store := &mutateStore[validatingConfig]{
		loadSnapshot: validatingConfig{Name: "ok"},
		loadMeta:     state.Meta{SnapshotID: "snap-1", ETag: "v1"},
		loadOK:       true,
		saveReturn:   state.Meta{SnapshotID: "snap-2", ETag: "v2"},
	}

	resolver := state.Resolver[validatingConfig]{Store: store}
	ref := state.Ref{
		Domain: "notifications",
		Scope:  opts.NewScope("user", opts.ScopePriorityUser, opts.WithScopeMetadata(map[string]any{"user_id": "u42"})),
	}

	_, _, err := resolver.Mutate(context.Background(), ref, state.Meta{ETag: "v1"}, func(v *validatingConfig) error {
		v.Name = ""
		return nil
	})
	if err == nil || err.Error() != "name is required" {
		t.Fatalf("expected validation error, got %v", err)
	}
	if store.saveCalls != 0 {
		t.Fatalf("expected no save calls, got %d", store.saveCalls)
	}
}

func TestResolverMutatePropagatesMetaAndSnapshotID(t *testing.T) {
	store := &mutateStore[map[string]any]{
		loadSnapshot: map[string]any{
			"notifications": map[string]any{
				"email": map[string]any{"enabled": false},
			},
		},
		loadMeta:   state.Meta{SnapshotID: "snap-old", ETag: "v1"},
		loadOK:     true,
		saveReturn: state.Meta{SnapshotID: "snap-new", ETag: "v2"},
	}

	resolver := state.Resolver[map[string]any]{Store: store}
	ref := state.Ref{
		Domain: "notifications",
		Scope:  opts.NewScope("user", opts.ScopePriorityUser, opts.WithScopeMetadata(map[string]any{"user_id": "u42"})),
	}

	options, gotMeta, err := resolver.Mutate(context.Background(), ref, state.Meta{ETag: "v1"}, func(v *map[string]any) error {
		email := (*v)["notifications"].(map[string]any)["email"].(map[string]any)
		email["enabled"] = true
		return nil
	})
	if err != nil {
		t.Fatalf("mutate: %v", err)
	}
	if gotMeta.SnapshotID != "snap-new" || gotMeta.ETag != "v2" {
		t.Fatalf("expected saved meta snap-new/v2, got %q/%q", gotMeta.SnapshotID, gotMeta.ETag)
	}

	if store.saveCalls != 1 {
		t.Fatalf("expected 1 save call, got %d", store.saveCalls)
	}
	if store.savedMeta.SnapshotID != "snap-old" || store.savedMeta.ETag != "v1" {
		t.Fatalf("expected save meta snap-old/v1, got %q/%q", store.savedMeta.SnapshotID, store.savedMeta.ETag)
	}

	// Provenance should reflect the saved SnapshotID.
	_, trace, err := options.ResolveWithTrace("notifications.email.enabled")
	if err != nil {
		t.Fatalf("trace: %v", err)
	}
	if len(trace.Layers) != 1 {
		t.Fatalf("expected 1 trace layer, got %d", len(trace.Layers))
	}
	if trace.Layers[0].SnapshotID != "snap-new" || trace.Layers[0].Scope.Name != "user" {
		t.Fatalf("expected trace snapshot=snap-new scope=user, got snapshot=%q scope=%q", trace.Layers[0].SnapshotID, trace.Layers[0].Scope.Name)
	}

	doc, err := options.Schema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	if len(doc.Scopes) != 1 {
		t.Fatalf("expected 1 schema scope, got %d", len(doc.Scopes))
	}
	if doc.Scopes[0].SnapshotID != "snap-new" || doc.Scopes[0].Name != "user" {
		t.Fatalf("expected schema snapshot=snap-new scope=user, got snapshot=%q scope=%q", doc.Scopes[0].SnapshotID, doc.Scopes[0].Name)
	}
}

func TestResolverMutateETagMismatch(t *testing.T) {
	store := &mutateStore[validatingConfig]{
		loadSnapshot: validatingConfig{Name: "ok"},
		loadMeta:     state.Meta{SnapshotID: "snap-1", ETag: "v1"},
		loadOK:       true,
		saveReturn:   state.Meta{SnapshotID: "snap-2", ETag: "v2"},
	}

	resolver := state.Resolver[validatingConfig]{Store: store}
	ref := state.Ref{
		Domain: "notifications",
		Scope:  opts.NewScope("user", opts.ScopePriorityUser, opts.WithScopeMetadata(map[string]any{"user_id": "u42"})),
	}

	_, _, err := resolver.Mutate(context.Background(), ref, state.Meta{ETag: "v2"}, func(v *validatingConfig) error {
		v.Name = "still-ok"
		return nil
	})
	if err == nil || !errors.Is(err, state.ErrETagMismatch) {
		t.Fatalf("expected ErrETagMismatch, got %v", err)
	}
	if store.saveCalls != 0 {
		t.Fatalf("expected no save calls, got %d", store.saveCalls)
	}
}

