package openapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	opts "github.com/goliatone/opts"
)

func TestGeneratorFixtures(t *testing.T) {
	t.Parallel()

	cases := []string{
		"schema_openapi_basic.json",
		"schema_openapi_nested.json",
		"schema_openapi_empty_collections.json",
	}

	for _, name := range cases {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fx := loadFixture(t, name)
			generator := NewGenerator()

			doc, err := generator.Generate(fx.Snapshot)
			if err != nil {
				t.Fatalf("Generate returned error: %v", err)
			}
			if doc.Format != opts.SchemaFormatOpenAPI {
				t.Fatalf("expected format %q, got %q", opts.SchemaFormatOpenAPI, doc.Format)
			}

			got, ok := doc.Document.(map[string]any)
			if !ok {
				t.Fatalf("expected schema document map[string]any, got %T", doc.Document)
			}

			if !reflect.DeepEqual(fx.Expect.Schema, got) {
				t.Fatalf("schema mismatch\nwant: %#v\ngot:  %#v", fx.Expect.Schema, got)
			}
		})
	}
}

func TestGeneratorNil(t *testing.T) {
	generator := NewGenerator()

	doc, err := generator.Generate(nil)
	if err != nil {
		t.Fatalf("Generate(nil) returned error: %v", err)
	}
	if doc.Format != opts.SchemaFormatOpenAPI {
		t.Fatalf("expected format %q, got %q", opts.SchemaFormatOpenAPI, doc.Format)
	}
	schema, ok := doc.Document.(map[string]any)
	if !ok {
		t.Fatalf("expected map document, got %T", doc.Document)
	}
	if schema["type"] != "null" {
		t.Fatalf("expected type null, got %v", schema["type"])
	}
}

type fixture struct {
	Snapshot map[string]any `json:"snapshot"`
	Expect   struct {
		Schema map[string]any `json:"schema"`
	} `json:"expect"`
}

func loadFixture(t *testing.T, name string) fixture {
	t.Helper()

	path := filepath.Join("testdata", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %q: %v", path, err)
	}

	var fx fixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("unmarshal fixture %q: %v", path, err)
	}
	return fx
}
