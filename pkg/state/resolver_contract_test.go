package state_test

import (
	"context"
	"testing"

	opts "github.com/goliatone/go-options"
	"github.com/goliatone/go-options/pkg/state"
)

type resolveFixture struct {
	Description string        `json:"description"`
	Cases       []resolveCase `json:"cases"`
}

type resolveCase struct {
	Name               string              `json:"name"`
	Domain             string              `json:"domain"`
	RequestedScopes    []fixtureScope      `json:"requested_scopes"`
	Records            []resolveRecord     `json:"records"`
	Resolve            []resolveAssertion  `json:"resolve"`
	SchemaScopesExpect []schemaScopeExpect `json:"schema_scopes_expect"`
}

type resolveRecord struct {
	Scope    fixtureScope   `json:"scope"`
	Meta     recordMeta     `json:"meta"`
	Snapshot map[string]any `json:"snapshot"`
}

type recordMeta struct {
	SnapshotID string `json:"snapshot_id"`
}

type resolveAssertion struct {
	Path   string          `json:"path"`
	Expect resolveExpected `json:"expect"`
}

type resolveExpected struct {
	Value any                  `json:"value"`
	Trace []expectedTraceLayer `json:"trace"`
}

type expectedTraceLayer struct {
	Scope      string `json:"scope"`
	Found      bool   `json:"found"`
	SnapshotID string `json:"snapshot_id"`
}

type schemaScopeExpect struct {
	Name       string `json:"name"`
	Priority   int    `json:"priority"`
	SnapshotID string `json:"snapshot_id"`
}

func TestResolverContracts(t *testing.T) {
	fx := loadFixture[resolveFixture](t, "state_resolve.json")
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

			options, err := resolver.Resolve(context.Background(), tc.Domain, scopes...)
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

func toOptsScope(s fixtureScope) opts.Scope {
	options := []opts.ScopeOption{}
	if s.Label != "" {
		options = append(options, opts.WithScopeLabel(s.Label))
	}
	if len(s.Metadata) > 0 {
		options = append(options, opts.WithScopeMetadata(asStringAnyMap(s.Metadata)))
	}
	return opts.NewScope(s.Name, s.Priority, options...)
}
