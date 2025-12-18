package state_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	opts "github.com/goliatone/go-options"
	"github.com/goliatone/go-options/pkg/state"
)

type identifierFixture struct {
	Description string           `json:"description"`
	Cases       []identifierCase `json:"cases"`
}

type identifierCase struct {
	Name   string        `json:"name"`
	Ref    identifierRef `json:"ref"`
	Expect expectValue   `json:"expect"`
}

type identifierRef struct {
	Domain string       `json:"domain"`
	Scope  fixtureScope `json:"scope"`
}

type fixtureScope struct {
	Name     string         `json:"name"`
	Label    string         `json:"label"`
	Priority int            `json:"priority"`
	Metadata map[string]any `json:"metadata"`
}

type expectValue struct {
	Value string `json:"value"`
	Err   string `json:"err"`
}

func TestRefIdentifierContracts(t *testing.T) {
	fx := loadFixture[identifierFixture](t, "state_identifier.json")
	for _, tc := range fx.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			ref := state.Ref{
				Domain: tc.Ref.Domain,
				Scope:  opts.NewScope(tc.Ref.Scope.Name, tc.Ref.Scope.Priority, opts.WithScopeMetadata(asStringAnyMap(tc.Ref.Scope.Metadata))),
			}
			got, err := ref.Identifier()

			if tc.Expect.Err != "" {
				if err == nil {
					t.Fatalf("expected error %q but got nil", tc.Expect.Err)
				}
				if err.Error() != tc.Expect.Err {
					t.Fatalf("expected error %q, got %q", tc.Expect.Err, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.Expect.Value {
				t.Fatalf("expected %q, got %q", tc.Expect.Value, got)
			}
		})
	}
}

func asStringAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func loadFixture[T any](t *testing.T, name string) T {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to locate fixture directory")
	}
	fixturePath := filepath.Join(filepath.Dir(filename), "..", "..", "testdata", name)
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture %q: %v", fixturePath, err)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("failed to unmarshal fixture %q: %v", fixturePath, err)
	}
	return out
}
