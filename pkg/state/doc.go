// Package state defines persistence-facing contracts for loading and saving
// per-scope option snapshots, plus a small resolver that orchestrates scope
// loading and delegates layering/provenance to the core go-options primitives.
//
// Responsibilities (per STATE_SPEC.md and ARCH_DESIGN.md):
//   - Store[T] only loads/saves a single snapshot for a single Ref.
//   - Resolver[T] loads snapshots for multiple scopes and merges them by
//     constructing opts.Layer[T] + opts.Stack[T].
//   - The core opts package remains persistence-agnostic; all persistence logic
//     stays behind Store implementations supplied by consumers.
//
// Data flow:
//
//	Store -> Resolver -> opts.NewStack(...).Merge(...) -> *opts.Options[T]
//
// Provenance:
//
//	Meta.SnapshotID is mapped onto opts.Layer[T].SnapshotID (via opts.WithSnapshotID),
//	which is then observable through Options.ResolveWithTrace(...) and (when enabled)
//	SchemaDocument.Scopes.
//
// Deterministic keys:
//
//	Ref.Identifier() provides a canonical storage key format based on the unified
//	scope model (`system/tenant/org/team/user`). If you previously persisted keys
//	using the legacy `global/group/user` prefixes from `github.com/goliatone/go-options/layering`,
//	handle backward compatibility in your adapter (e.g., read-old/write-new during migration).
package state
