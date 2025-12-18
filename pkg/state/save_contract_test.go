package state_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/goliatone/go-options/pkg/state"
)

type saveFixture struct {
	Description string     `json:"description"`
	Cases       []saveCase `json:"cases"`
}

type saveCase struct {
	Name   string `json:"name"`
	Ref    struct {
		Domain string       `json:"domain"`
		Scope  fixtureScope `json:"scope"`
	} `json:"ref"`
	Save struct {
		Snapshot map[string]any `json:"snapshot"`
		Meta     state.Meta     `json:"meta"`
	} `json:"save"`
	Expect struct {
		Meta         state.Meta     `json:"meta"`
		LoadOK       bool           `json:"load_ok"`
		LoadedMeta   state.Meta     `json:"loaded_meta"`
		LoadedRecord map[string]any `json:"loaded_snapshot"`
	} `json:"expect"`
}

func TestStoreSaveContracts(t *testing.T) {
	fx := loadFixture[saveFixture](t, "state_save.json")
	for _, tc := range fx.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			store := newMemoryStore[map[string]any]()
			ref := state.Ref{Domain: tc.Ref.Domain, Scope: toOptsScope(tc.Ref.Scope)}

			// Ensure any pre-existing record is overwritten.
			store.put(memoryStoreKey(ref), map[string]any{"_": "old"}, state.Meta{SnapshotID: "old", ETag: "old"})

			gotMeta, err := store.Save(context.Background(), ref, tc.Save.Snapshot, tc.Save.Meta)
			if err != nil {
				t.Fatalf("save: %v", err)
			}
			if diff := cmpJSON(tc.Expect.Meta, gotMeta); diff != "" {
				t.Fatalf("save meta mismatch: %s", diff)
			}

			gotSnapshot, gotLoadedMeta, ok, err := store.Load(context.Background(), ref)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if ok != tc.Expect.LoadOK {
				t.Fatalf("expected ok=%t, got ok=%t", tc.Expect.LoadOK, ok)
			}
			if !ok {
				return
			}

			if diff := cmpJSON(tc.Expect.LoadedMeta, gotLoadedMeta); diff != "" {
				t.Fatalf("load meta mismatch: %s", diff)
			}
			if diff := cmpJSON(tc.Expect.LoadedRecord, gotSnapshot); diff != "" {
				t.Fatalf("load snapshot mismatch: %s", diff)
			}
		})
	}
}

func cmpJSON(want, got any) string {
	wantRaw, err := json.Marshal(want)
	if err != nil {
		return "marshal want: " + err.Error()
	}
	gotRaw, err := json.Marshal(got)
	if err != nil {
		return "marshal got: " + err.Error()
	}
	if string(wantRaw) == string(gotRaw) {
		return ""
	}
	return "want=" + string(wantRaw) + " got=" + string(gotRaw)
}

