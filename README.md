# go-options

Lightweight helpers for wrapping application specific option structs with defaulting, validation, dynamic access helpers, and expression based rule evaluation. The package provides an `Evaluator` seam with adapters for expr-lang/expr, CEL-go, and goja (JavaScript) plus dynamic access helpers (`Get`, `Set`, `Schema`) for map or struct backed snapshots.

## Installation

```bash
go get github.com/goliatone/opts
```

## Quick Start

```go
package main

import (
	"fmt"

	opts "github.com/goliatone/opts"
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
}

wrapper := opts.New(
	map[string]any{
		"features": map[string]any{"newUI": false},
	},
	opts.WithFunctionRegistry(registry),
)

resp, _ := wrapper.EvaluateWith(ctx, `call("equalsIgnoreCase", metadata.expected, metadata.actual)`)
// resp.Value == true
```

## Rule Context

`RuleContext` carries:

- `Snapshot any` – evaluation target (defaults to the wrapped value).
- `Now *time.Time` – defaults to `time.Now()` when omitted.
- `Args map[string]any` – ad hoc inputs you want available to expressions.
- `Metadata map[string]any` – auxiliary information (for logging, audit, etc.).
- `Scope string` – optional label for the evaluation context.

All fields default to zero cost empty values so existing call sites continue to work unchanged.

## Dynamic Paths & Schema

The wrapper offers dynamic helpers while keeping direct struct access the primary path:

```go
wrapper := opts.New(map[string]any{
	"channels": map[string]any{
		"email": map[string]any{"enabled": true},
	},
})

enabled, _ := wrapper.Get("channels.email.enabled")
// enabled == true

_ = wrapper.Set("channels.push.enabled", true) // lazily creates intermediate maps

schema := wrapper.Schema()
for _, field := range schema.Fields {
	fmt.Printf("%s => %s\n", field.Path, field.Type)
}
```
Key details:
- `Get` traverses maps with string keys, exported struct fields, or fields tagged with `json:"name"`. It also supports slice/array indices (`items.0.id`).
- `Set` mutates map backed snapshots and lazily creates intermediate maps. Struct backed values are read only; attempting to call `Set` with a struct snapshot returns an error.
- `Schema` emits a flattened list of paths inferred from the snapshot. Basic primitives (`bool`, `string`, numeric types, slices) are normalised, for example `[]string` becomes `[]string`, and `int32` is reported as `int`. Unknown or composite types fall back to their Go type names.

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
```

## Evaluator Logging

Attach a logger to track evaluation events:

```go
logger := opts.EvaluatorLoggerFunc(func(event opts.EvaluatorLogEvent) {
	log.Printf("[%s] expr=%q scope=%q duration=%v err=%v",
		event.Engine, event.Expr, event.Scope, event.Duration, event.Err)
})

wrapper := opts.New(snapshot, opts.WithEvaluatorLogger(logger))
```

## Stability Guarantees

The exported surface sticks to functional options such as `WithEvaluator`, leaving room for additive hooks like `WithProgramCache` without breaking callers. `Options[T]`, `Response[T]`, and `RuleContext` provide the baseline API; future fields will default gracefully (e.g., `RuleContext.Now`) so existing code keeps compiling.

## Future Work

See [docs/TDD_OPTIONS.md](docs/TDD_OPTIONS.md) for the roadmap covering additional evaluators, caching hooks, dynamic access helpers, and schema generation. The public API revolves around functional options like `WithEvaluator`, `WithProgramCache`, and `WithFunctionRegistry` to keep future enhancements backwards compatible. All new capabilities will remain additive to preserve the stability guarantees.
