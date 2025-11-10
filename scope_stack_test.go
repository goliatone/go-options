package opts

import (
	"errors"
	"testing"
)

type sampleSnapshot struct {
	Name   string
	Count  *int
	Labels map[string]string
}

func intPtr(v int) *int {
	return &v
}

func TestNewScopeCopiesMetadata(t *testing.T) {
	meta := map[string]any{"owner": "system"}
	scope := NewScope("system", 50,
		WithScopeLabel("System Defaults"),
		WithScopeMetadata(meta),
	)

	meta["owner"] = "mutated"

	if got := scope.Metadata["owner"]; got != "system" {
		t.Fatalf("expected metadata copy to remain 'system', got %q", got)
	}
	if scope.Label != "System Defaults" {
		t.Fatalf("label not set, got %q", scope.Label)
	}
}

func TestNewLayerClonesSnapshot(t *testing.T) {
	snapshot := sampleSnapshot{
		Name:  "default",
		Count: intPtr(5),
		Labels: map[string]string{
			"env": "prod",
		},
	}

	layer := NewLayer(NewScope("user", 100), snapshot, WithSnapshotID[sampleSnapshot]("abc-123"))

	snapshot.Labels["env"] = "qa"
	if layer.Snapshot.Labels["env"] != "prod" {
		t.Fatalf("expected layer snapshot to remain immutable; got %q", layer.Snapshot.Labels["env"])
	}
	layer.Snapshot.Labels["env"] = "staging"
	if snapshot.Labels["env"] != "qa" {
		t.Fatalf("mutating layer snapshot should not affect original, got %q", snapshot.Labels["env"])
	}
	if layer.SnapshotID != "abc-123" {
		t.Fatalf("snapshot id not set, got %q", layer.SnapshotID)
	}
}

func TestNewStackOrdersAndValidates(t *testing.T) {
	user := NewLayer(NewScope("user", 300), sampleSnapshot{Name: "user"})
	group := NewLayer(NewScope("group", 200), sampleSnapshot{Name: "group"})
	defaults := NewLayer(NewScope("defaults", 100), sampleSnapshot{Name: "defaults"})

	stack, err := NewStack(defaults, user, group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	layers := stack.Layers()
	wantOrder := []string{"user", "group", "defaults"}
	for i, want := range wantOrder {
		if layers[i].Scope.Name != want {
			t.Fatalf("expected layer %d to be %q, got %q", i, want, layers[i].Scope.Name)
		}
	}

	if _, err := NewStack(user, NewLayer(NewScope("user", 50), sampleSnapshot{})); !errors.Is(err, ErrDuplicateScopeName) {
		t.Fatalf("expected duplicate scope name error, got %v", err)
	}

	if _, err := NewStack(
		NewLayer(NewScope("alpha", 100), sampleSnapshot{}),
		NewLayer(NewScope("beta", 100), sampleSnapshot{}),
	); !errors.Is(err, ErrPriorityOrder) {
		t.Fatalf("expected priority order error, got %v", err)
	}
}

func TestStackMergeStructSnapshots(t *testing.T) {
	defaults := NewLayer(NewScope("defaults", 100), sampleSnapshot{
		Name:  "defaults",
		Count: intPtr(3),
		Labels: map[string]string{
			"env": "prod",
		},
	})
	group := NewLayer(NewScope("group", 200), sampleSnapshot{
		Name:  "group",
		Count: intPtr(7),
	})
	user := NewLayer(NewScope("user", 300), sampleSnapshot{
		Name: "user",
		Labels: map[string]string{
			"team": "core",
		},
	})

	stack, err := NewStack(defaults, user, group)
	if err != nil {
		t.Fatalf("stack validation failed: %v", err)
	}

	merged, err := stack.Merge()
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	if merged.Value.Count == nil || *merged.Value.Count != 7 {
		t.Fatalf("expected Count pointer set to 7, got %+v", merged.Value.Count)
	}
	if merged.Value.Labels["env"] != "prod" || merged.Value.Labels["team"] != "core" {
		t.Fatalf("expected merged labels to combine maps, got %+v", merged.Value.Labels)
	}
}

func TestStackMergeMapSnapshots(t *testing.T) {
	type snapshot map[string]any
	defaults := NewLayer(NewScope("defaults", 10), snapshot{"dark_mode": false})
	user := NewLayer(NewScope("user", 20), snapshot{"dark_mode": true})

	stack, err := NewStack(defaults, user)
	if err != nil {
		t.Fatalf("stack validation failed: %v", err)
	}

	merged, err := stack.Merge()
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}
	if merged.Value["dark_mode"] != true {
		t.Fatalf("expected user override to win, got %+v", merged.Value)
	}
}

func TestStackLayersAreImmutable(t *testing.T) {
	stack, err := NewStack(
		NewLayer(NewScope("a", 100, WithScopeMetadata(map[string]any{"owner": "a"})),
			sampleSnapshot{Labels: map[string]string{"key": "value"}}),
		NewLayer(NewScope("b", 50), sampleSnapshot{}),
	)
	if err != nil {
		t.Fatalf("stack validation failed: %v", err)
	}

	layers := stack.Layers()
	layers[0].Scope.Metadata["owner"] = "mutated"
	layers[0].Snapshot.Labels["key"] = "mutated"

	next := stack.Layers()
	if next[0].Scope.Metadata["owner"] != "a" {
		t.Fatalf("expected metadata copy to remain 'a', got %q", next[0].Scope.Metadata["owner"])
	}
	if next[0].Snapshot.Labels["key"] != "value" {
		t.Fatalf("expected snapshot labels to remain 'value', got %q", next[0].Snapshot.Labels["key"])
	}
}

func TestStackLenAndEmpty(t *testing.T) {
	stack, err := NewStack[sampleSnapshot]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stack.Len() != 0 {
		t.Fatalf("empty stack len expected 0, got %d", stack.Len())
	}

	if _, err := stack.Merge(); err == nil {
		t.Fatalf("expected merge to fail for empty stack")
	}
	if layers := stack.Layers(); layers != nil {
		t.Fatalf("expected nil layers for empty stack, got %+v", layers)
	}
}
