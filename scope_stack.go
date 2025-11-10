package opts

import (
	"errors"
	"fmt"
	"sort"

	layering "github.com/goliatone/go-options/layering"
)

// Scope models a named precedence bucket (system, tenant, user, etc.). Higher
// priority values represent stronger layers.
type Scope struct {
	Name     string
	Label    string
	Priority int
	Metadata map[string]any
}

// ScopeOption configures metadata on Scope creation.
type ScopeOption func(*scopeConfig)

type scopeConfig struct {
	label    string
	metadata map[string]any
}

// WithScopeLabel sets a human-friendly label on the scope.
func WithScopeLabel(label string) ScopeOption {
	return func(cfg *scopeConfig) {
		cfg.label = label
	}
}

// WithScopeMetadata attaches arbitrary metadata to the scope. The map is copied
// so the resulting Scope remains immutable even if the caller mutates their
// reference.
func WithScopeMetadata(metadata map[string]any) ScopeOption {
	return func(cfg *scopeConfig) {
		if len(metadata) == 0 {
			return
		}
		cfg.metadata = copyMetadata(metadata)
	}
}

// NewScope builds a Scope with the supplied configuration. Validation is
// deferred to Stack construction so callers can assemble scopes before deciding
// precedence.
func NewScope(name string, priority int, opts ...ScopeOption) Scope {
	cfg := scopeConfig{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(&cfg)
	}
	return Scope{
		Name:     name,
		Label:    cfg.label,
		Priority: priority,
		Metadata: copyMetadata(cfg.metadata),
	}
}

// clone returns a copy of s, ensuring Metadata is detached from the original.
func (s Scope) clone() Scope {
	return Scope{
		Name:     s.Name,
		Label:    s.Label,
		Priority: s.Priority,
		Metadata: copyMetadata(s.Metadata),
	}
}

func (s Scope) isZero() bool {
	return s.Name == "" && s.Label == "" && s.Priority == 0 && len(s.Metadata) == 0
}

// Layer pairs a scope definition with the snapshot captured for that scope.
type Layer[T any] struct {
	Scope      Scope
	Snapshot   T
	SnapshotID string
}

// LayerOption configures optional metadata for a layer.
type LayerOption[T any] func(*Layer[T])

// WithSnapshotID sets the snapshot identifier used for auditing.
func WithSnapshotID[T any](id string) LayerOption[T] {
	return func(layer *Layer[T]) {
		layer.SnapshotID = id
	}
}

// NewLayer constructs a Layer with immutable copies of both the scope metadata
// and snapshot payload.
func NewLayer[T any](scope Scope, snapshot T, opts ...LayerOption[T]) Layer[T] {
	layer := Layer[T]{
		Scope:    scope.clone(),
		Snapshot: layering.Clone(snapshot),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(&layer)
	}
	return layer
}

var (
	// ErrScopeNameRequired indicates a missing scope name.
	ErrScopeNameRequired = errors.New("scope: name must be provided")
	// ErrDuplicateScopeName indicates Stack construction received multiple
	// layers with the same scope name.
	ErrDuplicateScopeName = errors.New("scope: names must be unique")
	// ErrPriorityOrder indicates Stack construction detected duplicate or
	// unsorted priorities.
	ErrPriorityOrder = errors.New("scope: priorities must be strictly ordered")
)

// Stack represents an immutable, scope-aware layering configuration ordered
// from strongest to weakest precedence.
type Stack[T any] struct {
	layers []Layer[T]
}

// NewStack validates and sorts the supplied layers so that the strongest scope
// (highest priority) is first. Layers and their snapshots are deep copied to
// guarantee read-only safety after construction.
func NewStack[T any](layers ...Layer[T]) (*Stack[T], error) {
	if len(layers) == 0 {
		return &Stack[T]{}, nil
	}

	seenNames := make(map[string]struct{}, len(layers))
	copied := make([]Layer[T], len(layers))
	for i, layer := range layers {
		layer := cloneLayer(layer)
		if layer.Scope.Name == "" {
			return nil, ErrScopeNameRequired
		}
		if _, ok := seenNames[layer.Scope.Name]; ok {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateScopeName, layer.Scope.Name)
		}
		seenNames[layer.Scope.Name] = struct{}{}
		copied[i] = layer
	}

	sort.Slice(copied, func(i, j int) bool {
		if copied[i].Scope.Priority == copied[j].Scope.Priority {
			return copied[i].Scope.Name < copied[j].Scope.Name
		}
		return copied[i].Scope.Priority > copied[j].Scope.Priority
	})

	for i := 1; i < len(copied); i++ {
		if copied[i-1].Scope.Priority <= copied[i].Scope.Priority {
			return nil, fmt.Errorf("%w: %d", ErrPriorityOrder, copied[i].Scope.Priority)
		}
	}

	return &Stack[T]{layers: copied}, nil
}

// Layers returns a defensive copy of the underlying layers to preserve
// immutability guarantees.
func (s *Stack[T]) Layers() []Layer[T] {
	if s == nil || len(s.layers) == 0 {
		return nil
	}
	out := make([]Layer[T], len(s.layers))
	for i := range s.layers {
		out[i] = cloneLayer(s.layers[i])
	}
	return out
}

// Len returns the number of layers in the stack.
func (s *Stack[T]) Len() int {
	if s == nil {
		return 0
	}
	return len(s.layers)
}

// Merge resolves the stack into an Options wrapper that retains provenance
// metadata for each contributing layer. The provided Option arguments apply to
// the resulting wrapper.
func (s *Stack[T]) Merge(opts ...Option) (*Options[T], error) {
	if s == nil || len(s.layers) == 0 {
		return nil, fmt.Errorf("scope: stack must include at least one layer")
	}
	snapshots := make([]T, len(s.layers))
	layerMeta := make([]layerSnapshot, len(s.layers))
	for i := range s.layers {
		snapshots[i] = layering.Clone(s.layers[i].Snapshot)
		layerMeta[i] = layerSnapshot{
			Scope:      s.layers[i].Scope.clone(),
			Snapshot:   layering.Clone(s.layers[i].Snapshot),
			SnapshotID: s.layers[i].SnapshotID,
		}
	}
	merged := layering.MergeLayers(snapshots...)
	options := New(merged, opts...)
	options.attachLayers(layerMeta)
	return options, nil
}

func cloneLayer[T any](layer Layer[T]) Layer[T] {
	return Layer[T]{
		Scope:      layer.Scope.clone(),
		Snapshot:   layering.Clone(layer.Snapshot),
		SnapshotID: layer.SnapshotID,
	}
}

type layerSnapshot struct {
	Scope      Scope
	Snapshot   any
	SnapshotID string
}

func copyMetadata(origin map[string]any) map[string]any {
	if len(origin) == 0 {
		return nil
	}
	out := make(map[string]any, len(origin))
	for key, value := range origin {
		out[key] = value
	}
	return out
}
