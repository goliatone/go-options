package state

import (
	"context"
	"errors"
	"fmt"
	"time"

	opts "github.com/goliatone/go-options"
)

var ErrNotImplemented = errors.New("state: not implemented")

var ErrETagMismatch = errors.New("state: etag mismatch")

// Ref identifies one persisted snapshot for one options domain.
type Ref struct {
	Domain string
	Scope  opts.Scope
}

// Meta is storage-owned metadata used for trace/audit and concurrency control.
type Meta struct {
	SnapshotID string            `json:"snapshot_id,omitempty"`
	ETag       string            `json:"etag,omitempty"`
	UpdatedAt  time.Time         `json:"updated_at,omitempty"`
	Extra      map[string]string `json:"extra,omitempty"`
}

// Store loads/saves one snapshot for a single scope reference.
type Store[T any] interface {
	Load(ctx context.Context, ref Ref) (snapshot T, meta Meta, ok bool, err error)
	Save(ctx context.Context, ref Ref, snapshot T, meta Meta) (Meta, error)
}

// Resolver orchestrates scoped loads and merges them into a single Options wrapper.
type Resolver[T any] struct {
	Store Store[T]
}

type Mutator[T any] func(*T) error

func (r Ref) Identifier() (string, error) {
	switch r.Scope.Name {
	case "system":
		return fmt.Sprintf("system/%s", r.Domain), nil
	case "tenant", "org", "team", "user":
		metadataKey := r.Scope.Name + "_id"
		id, ok := r.Scope.Metadata[metadataKey]
		if !ok {
			return "", fmt.Errorf("missing metadata key %q for scope %q", metadataKey, r.Scope.Name)
		}
		idString, ok := id.(string)
		if !ok || idString == "" {
			return "", fmt.Errorf("missing metadata key %q for scope %q", metadataKey, r.Scope.Name)
		}
		return fmt.Sprintf("%s/%s/%s", r.Scope.Name, idString, r.Domain), nil
	default:
		return "", fmt.Errorf("unsupported scope name %q", r.Scope.Name)
	}
}

func (r Resolver[T]) Resolve(ctx context.Context, domain string, scopes ...opts.Scope) (*opts.Options[T], error) {
	if r.Store == nil {
		return nil, fmt.Errorf("state: store is required")
	}
	if domain == "" {
		return nil, fmt.Errorf("state: domain is required")
	}
	if len(scopes) == 0 {
		return nil, fmt.Errorf("state: at least one scope is required")
	}

	layers := make([]opts.Layer[T], 0, len(scopes))
	for _, scope := range scopes {
		snapshot, meta, ok, err := r.Store.Load(ctx, Ref{Domain: domain, Scope: scope})
		if err != nil {
			return nil, fmt.Errorf("state: load %q for scope %q: %w", domain, scope.Name, err)
		}
		if !ok {
			continue
		}
		layers = append(layers, opts.NewLayer(scope, snapshot, opts.WithSnapshotID[T](meta.SnapshotID)))
	}

	if len(layers) == 0 {
		return nil, fmt.Errorf("state: no layers found for domain %q", domain)
	}

	stack, err := opts.NewStack(layers...)
	if err != nil {
		return nil, fmt.Errorf("state: stack: %w", err)
	}
	return stack.Merge(opts.WithScopeSchema(true))
}

func (r Resolver[T]) ResolveWithDefaults(ctx context.Context, domain string, defaults T, scopes ...opts.Scope) (*opts.Options[T], error) {
	if r.Store == nil {
		return nil, fmt.Errorf("state: store is required")
	}
	if domain == "" {
		return nil, fmt.Errorf("state: domain is required")
	}

	prioritySet := make(map[int]struct{}, len(scopes)+1)
	minPriority := 0
	if len(scopes) > 0 {
		minPriority = scopes[0].Priority
	}
	for _, scope := range scopes {
		if scope.Name == "defaults" {
			return nil, fmt.Errorf("state: scope name %q is reserved", "defaults")
		}
		prioritySet[scope.Priority] = struct{}{}
		if scope.Priority < minPriority {
			minPriority = scope.Priority
		}
	}

	defaultsPriority := 0
	if len(scopes) > 0 {
		defaultsPriority = minPriority - 1
		for {
			if _, ok := prioritySet[defaultsPriority]; !ok {
				break
			}
			defaultsPriority--
		}
	}

	layers := make([]opts.Layer[T], 0, len(scopes)+1)
	for _, scope := range scopes {
		snapshot, meta, ok, err := r.Store.Load(ctx, Ref{Domain: domain, Scope: scope})
		if err != nil {
			return nil, fmt.Errorf("state: load %q for scope %q: %w", domain, scope.Name, err)
		}
		if !ok {
			continue
		}
		layers = append(layers, opts.NewLayer(scope, snapshot, opts.WithSnapshotID[T](meta.SnapshotID)))
	}

	defaultsScope := opts.NewScope("defaults", defaultsPriority, opts.WithScopeLabel("Defaults"))
	layers = append(layers, opts.NewLayer(defaultsScope, defaults))

	stack, err := opts.NewStack(layers...)
	if err != nil {
		return nil, fmt.Errorf("state: stack: %w", err)
	}
	return stack.Merge(opts.WithScopeSchema(true))
}

// Mutate loads one snapshot, applies fn, validates via opts.Load, then saves.
func (r Resolver[T]) Mutate(ctx context.Context, ref Ref, meta Meta, fn Mutator[T]) (*opts.Options[T], Meta, error) {
	if r.Store == nil {
		return nil, Meta{}, fmt.Errorf("state: store is required")
	}
	if ref.Domain == "" {
		return nil, Meta{}, fmt.Errorf("state: domain is required")
	}
	if ref.Scope.Name == "" {
		return nil, Meta{}, fmt.Errorf("state: scope name is required")
	}
	if fn == nil {
		return nil, Meta{}, fmt.Errorf("state: mutator is required")
	}

	snapshot, loadedMeta, ok, err := r.Store.Load(ctx, ref)
	if err != nil {
		return nil, Meta{}, fmt.Errorf("state: load %q for scope %q: %w", ref.Domain, ref.Scope.Name, err)
	}
	if !ok {
		var zero T
		snapshot = zero
		loadedMeta = Meta{}
	}

	if meta.ETag != "" && loadedMeta.ETag != "" && meta.ETag != loadedMeta.ETag {
		return nil, loadedMeta, fmt.Errorf("%w: expected %q, got %q", ErrETagMismatch, meta.ETag, loadedMeta.ETag)
	}

	if err := fn(&snapshot); err != nil {
		return nil, loadedMeta, err
	}

	if _, err := opts.Load(snapshot); err != nil {
		return nil, loadedMeta, err
	}

	saveMeta := mergeMeta(loadedMeta, meta)
	savedMeta, err := r.Store.Save(ctx, ref, snapshot, saveMeta)
	if err != nil {
		return nil, loadedMeta, fmt.Errorf("state: save %q for scope %q: %w", ref.Domain, ref.Scope.Name, err)
	}

	layer := opts.NewLayer(ref.Scope, snapshot, opts.WithSnapshotID[T](savedMeta.SnapshotID))
	stack, err := opts.NewStack(layer)
	if err != nil {
		return nil, loadedMeta, fmt.Errorf("state: stack: %w", err)
	}
	options, err := stack.Merge(opts.WithScopeSchema(true))
	if err != nil {
		return nil, loadedMeta, err
	}
	return options, savedMeta, nil
}

func mergeMeta(base, override Meta) Meta {
	out := base
	if override.SnapshotID != "" {
		out.SnapshotID = override.SnapshotID
	}
	if override.ETag != "" {
		out.ETag = override.ETag
	}
	if !override.UpdatedAt.IsZero() {
		out.UpdatedAt = override.UpdatedAt
	}
	if override.Extra != nil {
		out.Extra = override.Extra
	}
	return out
}
