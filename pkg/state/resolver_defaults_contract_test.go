package state_test

import (
	"context"
	"testing"

	opts "github.com/goliatone/go-options"
	"github.com/goliatone/go-options/pkg/state"
)

type resolveWithDefaultsFixture struct {
	Description string                    `json:"description"`
	Cases       []resolveWithDefaultsCase `json:"cases"`
}

type resolveWithDefaultsCase struct {
	Name               string              `json:"name"`
	Domain             string              `json:"domain"`
	Defaults           map[string]any      `json:"defaults"`
	RequestedScopes    []fixtureScope      `json:"requested_scopes"`
	Records            []resolveRecord     `json:"records"`
	Resolve            []resolveAssertion  `json:"resolve"`
	SchemaScopesExpect []schemaScopeExpect `json:"schema_scopes_expect"`
}

func TestResolverResolveWithDefaultsContracts(t *testing.T) {
	fx := loadFixture[resolveWithDefaultsFixture](t, "state_resolve_defaults.json")
	for _, tc := range fx.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			store := newMemoryStore[map[string]any]()
			for _, rec := range tc.Records {
				scope := toOptsScope(rec.Scope)
				ref := state.Ref{Domain: tc.Domain, Scope: scope}
				store.put(memoryStoreKey(ref), rec.Snapshot, state.Meta{SnapshotID: rec.Meta.SnapshotID})
			}

			resolver := state.Resolver[map[string]any]{Store: store}

			scopes := make([]opts.Scope, 0, len(tc.RequestedScopes))
			for _, s := range tc.RequestedScopes {
				scopes = append(scopes, toOptsScope(s))
			}

			options, err := resolver.ResolveWithDefaults(context.Background(), tc.Domain, tc.Defaults, scopes...)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}

			for _, assertion := range tc.Resolve {
				value, trace, err := options.ResolveWithTrace(assertion.Path)
				if err != nil {
					t.Fatalf("trace %q: %v", assertion.Path, err)
				}
				if value != assertion.Expect.Value {
					t.Fatalf("path %q expected value %v, got %v", assertion.Path, assertion.Expect.Value, value)
				}

				if len(trace.Layers) != len(assertion.Expect.Trace) {
					t.Fatalf("path %q expected %d trace layers, got %d", assertion.Path, len(assertion.Expect.Trace), len(trace.Layers))
				}
				for i, expected := range assertion.Expect.Trace {
					layer := trace.Layers[i]
					if layer.Scope.Name != expected.Scope || layer.Found != expected.Found || layer.SnapshotID != expected.SnapshotID {
						t.Fatalf("path %q layer[%d] expected scope=%q found=%t snapshot=%q, got scope=%q found=%t snapshot=%q",
							assertion.Path, i, expected.Scope, expected.Found, expected.SnapshotID, layer.Scope.Name, layer.Found, layer.SnapshotID)
					}
				}
			}

			doc, err := options.Schema()
			if err != nil {
				t.Fatalf("schema: %v", err)
			}
			if len(doc.Scopes) != len(tc.SchemaScopesExpect) {
				t.Fatalf("expected %d schema scopes, got %d", len(tc.SchemaScopesExpect), len(doc.Scopes))
			}
			for i, expected := range tc.SchemaScopesExpect {
				scope := doc.Scopes[i]
				if scope.Name != expected.Name || scope.Priority != expected.Priority || scope.SnapshotID != expected.SnapshotID {
					t.Fatalf("schema scope[%d] expected name=%q priority=%d snapshot=%q, got name=%q priority=%d snapshot=%q",
						i, expected.Name, expected.Priority, expected.SnapshotID, scope.Name, scope.Priority, scope.SnapshotID)
				}
			}
		})
	}
}

