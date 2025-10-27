package opts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestNewScopeChainOrderingFromFixture(t *testing.T) {
	fx := loadScopeChainFixture(t, "layering_scope_chain.json")

	for _, tc := range fx.Cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			input := make([]Scope, len(tc.Input))
			for i := range tc.Input {
				input[i] = scopeFromFixture(tc.Key, tc.Input[i])
			}

			chain := NewScopeChain(input...)
			got := chain.Ordered()

			expect := make([]Scope, len(tc.Expect))
			for i := range tc.Expect {
				expect[i] = scopeFromFixture(tc.Key, tc.Expect[i])
			}

			if !reflect.DeepEqual(expect, got) {
				t.Fatalf("unexpected layering order\nwant: %#v\n got: %#v", expect, got)
			}

			if len(expect) == 0 {
				if strongest := chain.Strongest(); strongest != (Scope{}) {
					t.Fatalf("expected zero strongest scope, got %#v", strongest)
				}
				if weakest := chain.Weakest(); weakest != (Scope{}) {
					t.Fatalf("expected zero weakest scope, got %#v", weakest)
				}
				return
			}

			if strongest := chain.Strongest(); strongest != expect[0] {
				t.Fatalf("expected strongest %#v, got %#v", expect[0], strongest)
			}
			if weakest := chain.Weakest(); weakest != expect[len(expect)-1] {
				t.Fatalf("expected weakest %#v, got %#v", expect[len(expect)-1], weakest)
			}
		})
	}
}

func TestScopeIdentifier(t *testing.T) {
	scope := Scope{
		Key:   "notifications",
		Level: ScopeLevelUser,
		User:  "user-99",
		Group: "group-2",
	}
	if got, want := scope.Identifier(), "user/user-99/notifications"; got != want {
		t.Fatalf("unexpected identifier: want %q got %q", want, got)
	}
	global := Scope{Key: "notifications", Level: ScopeLevelGlobal}
	if got, want := global.Identifier(), "global/notifications"; got != want {
		t.Fatalf("unexpected global identifier: want %q got %q", want, got)
	}
}

type scopeChainFixture struct {
	Description string                  `json:"description"`
	Cases       []scopeChainFixtureCase `json:"cases"`
}

type scopeChainFixtureCase struct {
	Name   string              `json:"name"`
	Key    string              `json:"key"`
	Input  []scopeFixtureScope `json:"input"`
	Expect []scopeFixtureScope `json:"expect"`
}

type scopeFixtureScope struct {
	Level string `json:"level"`
	User  string `json:"user"`
	Group string `json:"group"`
}

func loadScopeChainFixture(t *testing.T, name string) scopeChainFixture {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to resolve caller for fixture %q", name)
	}
	path := filepath.Join(filepath.Dir(file), "testdata", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read scope chain fixture %q: %v", name, err)
	}
	var fx scopeChainFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("failed to unmarshal scope chain fixture %q: %v", name, err)
	}
	return fx
}

func scopeFromFixture(key string, fx scopeFixtureScope) Scope {
	return Scope{
		Key:   key,
		Level: ParseScopeLevel(fx.Level),
		User:  fx.User,
		Group: fx.Group,
	}
}
