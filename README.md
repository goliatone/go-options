# go-options

Lightweight helpers for wrapping application specific option structs with defaulting, validation, dynamic access helpers, and expression based rule evaluation. The package provides an `Evaluator` seam with adapters for expr-lang/expr, CEL-go, and goja (JavaScript) plus dynamic access helpers (`Get`, `Set`, `Schema`) for map or struct backed snapshots.

## Installation

```bash
go get github.com/goliatone/go-options
```

## Quick Start

```go
package main

import (
	"fmt"

	opts "github.com/goliatone/go-options"
)

type NotificationOptions struct {
	Enabled bool
}

func main() {
	// Apply defaults.
	defaults := NotificationOptions{Enabled: true}
	current := opts.ApplyDefaults(NotificationOptions{}, defaults)

	// Wrap the snapshot and evaluate rules with the default expr adapter.
	wrapper := opts.New(
		map[string]any{
			"notifications": map[string]any{"enabled": current.Enabled},
		},
	)

	resp, err := wrapper.Evaluate("notifications.enabled")
	if err != nil {
		panic(err)
	}
fmt.Println("notifications.enabled:", resp.Value)
}
```

## Scope Quick Start

The new scope primitives make it trivial to build multi-tenant stacks, inspect provenance, and surface scope metadata in schemas. See `examples/scope/main.go` for a runnable version of the snippets below.

### Build a deterministic stack

```go
type Settings struct {
	Notifications map[string]any
}

defaults := opts.NewLayer(
	opts.NewScope("defaults", 0, opts.WithScopeLabel("System Defaults")),
	Settings{Notifications: map[string]any{"email": map[string]any{"enabled": false}}},
)
tenant := opts.NewLayer(
	opts.NewScope("tenant", 50, opts.WithScopeMetadata(map[string]any{"slug": "acme"})),
	Settings{Notifications: map[string]any{"email": map[string]any{"enabled": true}}},
	opts.WithSnapshotID[Settings]("tenant/acme"),
)
user := opts.NewLayer(
	opts.NewScope("user", 100, opts.WithScopeLabel("User Override")),
	Settings{Notifications: map[string]any{"email": map[string]any{"enabled": true}}},
	opts.WithSnapshotID[Settings]("user/42"),
)

stack, err := opts.NewStack(defaults, tenant, user)
if err != nil {
	log.Fatalf("stack: %v", err)
}
options, err := stack.Merge(opts.WithScopeSchema(true))
if err != nil {
	log.Fatalf("merge: %v", err)
}
```

### Resolve with trace

```go
value, trace, err := options.ResolveWithTrace("Notifications.Email.Enabled")
if err != nil {
	log.Fatalf("trace: %v", err)
}
fmt.Printf("Effective value: %v\n", value)
for _, layer := range trace.Layers {
	fmt.Printf("%s (priority=%d) found=%v snapshot=%s\n",
		layer.Scope.Name, layer.Scope.Priority, layer.Found, layer.SnapshotID)
}
```

`ResolveWithTrace` walks layers strongest → weakest so you can explain why a value resolved the way it did. `trace.Layers` mirrors the JSON payload returned by `Trace.ToJSON()`.

### Scope-aware schemas

```go
doc, err := options.Schema()
if err != nil {
	log.Fatalf("schema: %v", err)
}
for _, scope := range doc.Scopes {
	fmt.Printf("scope=%s priority=%d snapshot=%s\n", scope.Name, scope.Priority, scope.SnapshotID)
}
```

Because the stack was merged with `opts.WithScopeSchema(true)`, each schema now emits an ordered list of scopes alongside the regular descriptor/OpenAPI payload.

## Defaults & Validation

`ApplyDefaults` returns the fallback struct when the current value is the zero value. `Load` combines construction with a validation pass. If the wrapped struct (or its pointer form) implements `Validate() error`, the hook fires automatically.

```go
type ChannelOptions struct {
	Enabled bool
}

func (c ChannelOptions) Validate() error {
	if !c.Enabled {
		return errors.New("channel disabled")
	}
	return nil
}

wrapper, err := opts.Load(ChannelOptions{Enabled: false})
// err => "channel disabled"
```

## Rule Evaluation

Expressions run against the snapshot stored in `Options[T]`. `Evaluate(expr)` uses the wrapped value as the environment. `EvaluateWith(ctx, expr)` lets callers override the snapshot and timestamp for a single evaluation. Every evaluator exposes the helper `call("functionName", args...)` which routes through the shared function registry so custom helpers behave consistently across engines.

```go
registry := opts.NewFunctionRegistry()
_ = registry.Register("equalsIgnoreCase", func(args ...any) (any, error) {
	return strings.EqualFold(args[0].(string), args[1].(string)), nil
})

ctx := opts.RuleContext{
	Snapshot: map[string]any{
		"features": map[string]any{"newUI": true},
	},
	Metadata: map[string]any{
		"expected": "ENABLED",
		"actual":   "enabled",
	},
	Scope: opts.NewScope("tenant", opts.ScopePriorityTenant),
}

wrapper := opts.New(
	map[string]any{
		"features": map[string]any{"newUI": false},
	},
	opts.WithFunctionRegistry(registry),
	opts.WithScope(opts.NewScope("user:42", opts.ScopePriorityUser)),
)

resp, _ := wrapper.EvaluateWith(ctx, `scope.name == "tenant" && call("equalsIgnoreCase", metadata.expected, metadata.actual)`)
// resp.Value == true
```

## Rule Context

`RuleContext` carries:

- `Snapshot any` – evaluation target (defaults to the wrapped value).
- `Now *time.Time` – defaults to `time.Now()` when omitted.
- `Args map[string]any` – ad hoc inputs you want available to expressions.
- `Metadata map[string]any` – auxiliary information (for logging, audit, etc.).
- `Scope opts.Scope` – structured metadata describing the active layer; evaluators expose it to expressions as `scope`.
- `ScopeName string` – legacy string label retained for callers that only know the raw scope identifier.

Use `opts.WithScope(...)` when constructing an options wrapper to seed this metadata automatically; it will be copied into every `RuleContext` unless you override it per invocation.

All fields default to zero cost empty values so existing call sites continue to work unchanged.

## Dynamic Paths & Schema

The wrapper offers dynamic helpers while keeping direct struct access the primary path:

```go
import (
	openapi "github.com/goliatone/go-options/schema/openapi"
)

wrapper := opts.New(map[string]any{
	"channels": map[string]any{
		"email": map[string]any{"enabled": true},
	},
})

enabled, _ := wrapper.Get("channels.email.enabled")
// enabled == true

_ = wrapper.Set("channels.push.enabled", true) // lazily creates intermediate maps

doc := wrapper.MustSchema()
fields, ok := doc.Document.([]opts.FieldDescriptor)
if !ok {
	log.Fatalf("unexpected schema payload %T", doc.Document)
}
for _, field := range fields {
	fmt.Printf("%s => %s\n", field.Path, field.Type)
}

openAPIDoc, err := wrapper.WithSchemaGenerator(openapi.NewGenerator()).Schema()
if err != nil {
	log.Fatalf("openapi schema: %v", err)
}
fmt.Println(openAPIDoc.Format) // "openapi"
```

Key details:
- `Get` traverses maps with string keys, exported struct fields, or fields tagged with `json:"name"`. It also supports slice/array indices (`items.0.id`).
- `Set` mutates map backed snapshots and lazily creates intermediate maps. Struct backed values are read only; attempting to call `Set` with a struct snapshot returns an error.
- `Schema()` returns a `SchemaDocument` describing the wrapped value. The default generator emits flattened `FieldDescriptor` paths. Pass `opts.WithSchemaGenerator(...)` (or `schema/openapi.Option()`) to swap in alternate representations such as OpenAPI/JSON Schema.
- Opt into scope descriptors by merging stacks with `opts.WithScopeSchema(true)`; `SchemaDocument.Scopes` then lists every layer (name, label, priority, snapshot ID, metadata) alongside the generated schema.

### Schema Generators

`Options.Schema()` delegates to a configurable `SchemaGenerator`. The default generator produces a slice of `FieldDescriptor` values (format `descriptors`). To generate OpenAPI compatible schemas:

```go
import openapi "github.com/goliatone/go-options/schema/openapi"

wrapper := opts.New(snapshot, openapi.Option(
	openapi.WithInfo("Config Service", "1.2.0"),
	openapi.WithOperation("/config", "POST", "createConfig"),
	openapi.WithResponse("204", "Configuration accepted"),
))
doc, _ := wrapper.Schema()
if doc.Format == opts.SchemaFormatOpenAPI {
	fmt.Printf("properties: %#v\n", doc.Document)
}
```

The OpenAPI generator is configurable through functional options:

- `openapi.WithOpenAPIVersion("3.1.0")`
- `openapi.WithInfo("Service Title", "version", openapi.WithInfoDescription("..."))`
- `openapi.WithOperation("/path", "METHOD", "operationId", openapi.WithOperationSummary("..."))`
- `openapi.WithContentType("application/json")`
- `openapi.WithResponse("204", "Description")`

See `docs/SCHEMA_TDD.md` for the design background and future roadmap for schema exports.

Custom generators implement:

```go
type SchemaGenerator interface {
    Generate(value any) (opts.SchemaDocument, error)
}
```

Generators must be safe for concurrent use and handle `nil` inputs by returning an empty schema (usually `{ "type": "null" }`). Use `opts.WithSchemaGenerator(myGenerator)` or `opts.Options.WithSchemaGenerator(...)` to attach custom implementations per wrapper.

## Custom Functions & Shared Registry

Use `opts.WithCustomFunction(name, fn)` or `opts.WithFunctionRegistry(registry)` when constructing the wrapper. Functions accept a variadic `[]any` payload and may return `(any, error)`; returning an error propagates to the evaluator. Expressions call helpers via `call("functionName", args...)`. The expr adapter additionally exposes direct symbols for convenience.

```go
registry := opts.NewFunctionRegistry()
_ = registry.Register("withinQuietHours", func(args ...any) (any, error) {
	now := args[0].(time.Time)
	start := args[1].(time.Time)
	end := args[2].(time.Time)
	return (now.Equal(start) || now.After(start)) && now.Before(end), nil
})

wrapper := opts.New(snapshot,
	opts.WithFunctionRegistry(registry),
)

resp, err := wrapper.EvaluateWith(
	opts.RuleContext{Snapshot: snapshot, Now: &now},
	`call("withinQuietHours", now, QuietHours.Start, QuietHours.End)`,
)
```

## Evaluator Options & Caching

All evaluators share the same configuration surface:

- `opts.NewExprEvaluator(...)` – expr-lang/expr wrapper.
- `opts.NewCELEvaluator(...)` – CEL-go adapter.
- `opts.NewJSEvaluator(...)` – goja/JavaScript adapter (requires `js_eval` build tag).
- `opts.WithEvaluator` – plug the evaluator into the wrapper.
- `opts.WithProgramCache(cache)` – supply a memoisation layer for compiled programs (used by expr, CEL, and JS adapters).
- `ExprWithProgramCache`, `CELWithProgramCache`, and `JSWithProgramCache` wire caches directly when you build adapters manually.
- `ExprWithFunctionRegistry`, `CELWithFunctionRegistry`, and `JSWithFunctionRegistry` keep custom functions in sync.

When no evaluator is configured, `opts.New` defaults to the expr adapter automatically.

## Layering

`LayerWith` merges snapshots ordered strongest to weakest with the current snapshot as the fallback, returning a new wrapper with the merged value.

```go
defaults := AppOptions{Timeout: 30, Retries: 3}
groupDefaults := AppOptions{Timeout: 60}
userSettings := AppOptions{Retries: 5}

wrapper := opts.New(defaults)
merged := wrapper.LayerWith(userSettings, groupDefaults)
// merged.Value => AppOptions{Timeout: 60, Retries: 5}

// Canonical five-layer stack helper:
stacked, _ := opts.SystemTenantOrgTeamUser(defaults, tenant, org, team, userSettings)
```

## Scope Stacks & Tracing

Construct deterministic stacks with named scopes, merge them, then inspect provenance:

```go
stack, _ := opts.NewStack(
	opts.NewLayer(opts.NewScope("defaults", 0), DefaultSettings),
	opts.NewLayer(opts.NewScope("tenant", 50), TenantSettings, opts.WithSnapshotID[Settings]("tenant/acme")),
	opts.NewLayer(opts.NewScope("user", 100), UserSettings, opts.WithSnapshotID[Settings]("user/42")),
)
options, _ := stack.Merge()

value, trace, _ := options.ResolveWithTrace("Notifications.Email.Enabled")
fmt.Println(value)           // strongest layer's value
_ = json.NewEncoder(os.Stdout).Encode(trace)

provenance, _ := options.FlattenWithProvenance()
for _, entry := range provenance {
	fmt.Printf("%s => scope=%s snapshot=%s\n", entry.Path, entry.Scope.Name, entry.SnapshotID)
}
```

`ResolveWithTrace` walks every layer from strongest → weakest so you always know which scope supplied a value (or why it fell back). `FlattenWithProvenance` enumerates all reachable paths for documentation/debug tooling.

## Evaluator Logging

Attach a logger to track evaluation events:

```go
logger := opts.EvaluatorLoggerFunc(func(event opts.EvaluatorLogEvent) {
	log.Printf("[%s] expr=%q scope=%q duration=%v err=%v",
		event.Engine, event.Expr, event.Scope, event.Duration, event.Err)
})

wrapper := opts.New(snapshot, opts.WithEvaluatorLogger(logger))
```
