package opts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMergeLayersFromFixture(t *testing.T) {
	fx := loadLayeringFixture(t, "layering_merge.json")

	for _, tc := range fx.Cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			layers := make([]layeringSettings, len(tc.Layers))
			for i := range tc.Layers {
				layers[i] = tc.Layers[i].Snapshot
			}

			got := MergeLayers[layeringSettings](layers...)
			if !reflect.DeepEqual(tc.Expect, got) {
				t.Errorf("merged snapshot mismatch:\nwant: %#v\n got: %#v", tc.Expect, got)
			}
		})
	}
}

func TestMergeLayersZeroInput(t *testing.T) {
	type sample struct {
		Value int
	}
	var zero sample
	if got := MergeLayers[sample](); got != zero {
		t.Fatalf("expected MergeLayers() to return zero value, got %+v", got)
	}
}

type layeringFixture struct {
	Description string                `json:"description"`
	Cases       []layeringFixtureCase `json:"cases"`
}

type layeringFixtureCase struct {
	Name   string                 `json:"name"`
	Layers []layeringFixtureLayer `json:"layers"`
	Expect layeringSettings       `json:"expect"`
	Notes  string                 `json:"notes"`
}

type layeringFixtureLayer struct {
	Scope    string           `json:"scope"`
	Snapshot layeringSettings `json:"snapshot"`
}

type layeringSettings struct {
	Enabled   *bool                     `json:"enabled,omitempty"`
	Limits    map[string]int            `json:"limits,omitempty"`
	Channel   *layeringChannel          `json:"channel,omitempty"`
	Tags      []string                  `json:"tags,omitempty"`
	Threshold *int                      `json:"threshold,omitempty"`
	Metadata  map[string]any            `json:"metadata,omitempty"`
	Extras    map[string]layeringExtras `json:"extras,omitempty"`
}

type layeringChannel struct {
	Enabled *bool    `json:"enabled,omitempty"`
	Volume  *int     `json:"volume,omitempty"`
	Labels  []string `json:"labels,omitempty"`
}

type layeringExtras struct {
	Flags []string `json:"flags,omitempty"`
}

func loadLayeringFixture(t *testing.T, name string) layeringFixture {
	t.Helper()
	path := filepath.Join("testdata", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read layering fixture %q: %v", name, err)
	}
	var fx layeringFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("failed to unmarshal layering fixture %q: %v", name, err)
	}
	return fx
}
