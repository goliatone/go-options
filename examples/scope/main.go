package main

import (
	"encoding/json"
	"fmt"
	"log"

	opts "github.com/goliatone/opts"
)

type snapshot = map[string]any

func main() {
	fmt.Println("=== go-options scope demo ===")

	defaults := opts.NewLayer(
		opts.NewScope("system", opts.ScopePrioritySystem, opts.WithScopeLabel("System Defaults")),
		snapshot{
			"notifications": map[string]any{
				"email": map[string]any{
					"enabled": false,
					"subject": "System template",
				},
			},
			"schemaVersion": 1,
		},
	)
	tenant := opts.NewLayer(
		opts.NewScope("tenant", opts.ScopePriorityTenant, opts.WithScopeMetadata(map[string]any{"slug": "acme"})),
		snapshot{
			"notifications": map[string]any{
				"email": map[string]any{"enabled": true},
				"sms":   map[string]any{"enabled": true},
			},
		},
		opts.WithSnapshotID[snapshot]("tenant/acme"),
	)
	user := opts.NewLayer(
		opts.NewScope("user", opts.ScopePriorityUser, opts.WithScopeLabel("User Override")),
		snapshot{
			"notifications": map[string]any{
				"email": map[string]any{
					"enabled": true,
					"subject": "Welcome back",
				},
			},
		},
		opts.WithSnapshotID[snapshot]("user/42"),
	)

	stack := mustNewStack(defaults, tenant, user)
	options := mustMerge(stack, opts.WithScopeSchema(true), opts.WithScope(user.Scope))

	fmt.Println("\n-- Resolve with trace --")
	showResolveWithTrace(options)

	fmt.Println("\n-- Flatten with provenance --")
	showFlattenWithProvenance(options)

	fmt.Println("\n-- Schema scopes --")
	showScopeAwareSchema(options)

	fmt.Println("\n-- Evaluator hooks --")
	showEvaluatorScopes(options)
}

func mustNewStack(layers ...opts.Layer[snapshot]) *opts.Stack[snapshot] {
	stack, err := opts.NewStack(layers...)
	if err != nil {
		log.Fatalf("stack: %v", err)
	}
	return stack
}

func mustMerge(stack *opts.Stack[snapshot], options ...opts.Option) *opts.Options[snapshot] {
	optsWrapper, err := stack.Merge(options...)
	if err != nil {
		log.Fatalf("merge: %v", err)
	}
	return optsWrapper
}

func showResolveWithTrace(options *opts.Options[snapshot]) {
	value, trace, err := options.ResolveWithTrace("notifications.email.enabled")
	if err != nil {
		log.Fatalf("trace: %v", err)
	}
	fmt.Printf("notifications.email.enabled => %v\n", value)
	payload, _ := json.MarshalIndent(trace, "  ", "  ")
	fmt.Println(string(payload))
}

func showFlattenWithProvenance(options *opts.Options[snapshot]) {
	provenance, err := options.FlattenWithProvenance()
	if err != nil {
		log.Fatalf("flatten: %v", err)
	}
	for _, entry := range provenance {
		scopeName := entry.Scope.Name
		if scopeName == "" {
			scopeName = "(anonymous)"
		}
		fmt.Printf("%s => scope=%s found=%t snapshot=%s\n", entry.Path, scopeName, entry.Found, entry.SnapshotID)
	}
}

func showScopeAwareSchema(options *opts.Options[snapshot]) {
	doc, err := options.Schema()
	if err != nil {
		log.Fatalf("schema: %v", err)
	}
	fmt.Printf("schema format: %s\n", doc.Format)
	for _, scope := range doc.Scopes {
		fmt.Printf("scope=%s priority=%d snapshot=%s label=%s\n",
			scope.Name, scope.Priority, scope.SnapshotID, scope.Label)
	}
}

func showEvaluatorScopes(options *opts.Options[snapshot]) {
	resp, err := options.Evaluate(`notifications.email.enabled && scope.name == "user"`)
	if err != nil {
		log.Fatalf("evaluate (user): %v", err)
	}
	fmt.Printf("user scope expression => %v\n", resp.Value)

	ctx := opts.RuleContext{
		Scope: opts.NewScope("tenant-preview", opts.ScopePriorityTenant,
			opts.WithScopeMetadata(map[string]any{"slug": "acme"}),
		),
	}
	resp, err = options.EvaluateWith(ctx, `scope.metadata.slug == "acme" && notifications.sms.enabled`)
	if err != nil {
		log.Fatalf("evaluate (tenant preview): %v", err)
	}
	fmt.Printf("tenant preview expression => %v\n", resp.Value)
}
