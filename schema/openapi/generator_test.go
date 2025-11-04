package openapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	opts "github.com/goliatone/opts"
)

func TestNewGeneratorOptions(t *testing.T) {
	custom := NewGenerator(
		WithOpenAPIVersion("3.1.0"),
		WithInfo("Custom Service", "2.0.0", WithInfoDescription("custom schema")),
		WithOperation("/settings", "PUT", "updateSettings", WithOperationSummary("Update settings")),
		WithContentType("application/x-www-form-urlencoded"),
		WithResponse("201", "Created"),
	)

	internal, ok := custom.(generator)
	if !ok {
		t.Fatalf("expected generator implementation, got %T", custom)
	}

	if got := internal.config.openAPIVersion; got != "3.1.0" {
		t.Fatalf("expected openapi version 3.1.0, got %q", got)
	}
	if got := internal.config.info.Title; got != "Custom Service" {
		t.Fatalf("expected info title Custom Service, got %q", got)
	}
	if got := internal.config.info.Version; got != "2.0.0" {
		t.Fatalf("expected info version 2.0.0, got %q", got)
	}
	if got := internal.config.info.Description; got != "custom schema" {
		t.Fatalf("expected info description custom schema, got %q", got)
	}
	if got := internal.config.operation.Path; got != "/settings" {
		t.Fatalf("expected operation path /settings, got %q", got)
	}
	if got := internal.config.operation.Method; got != "put" {
		t.Fatalf("expected method put, got %q", got)
	}
	if got := internal.config.operation.OperationID; got != "updateSettings" {
		t.Fatalf("expected operation id updateSettings, got %q", got)
	}
	if got := internal.config.operation.Summary; got != "Update settings" {
		t.Fatalf("expected operation summary Update settings, got %q", got)
	}
	if got := internal.config.contentType; got != "application/x-www-form-urlencoded" {
		t.Fatalf("expected content type application/x-www-form-urlencoded, got %q", got)
	}
	if got := internal.config.responses["201"].Description; got != "Created" {
		t.Fatalf("expected response description Created, got %q", got)
	}
	if _, exists := internal.config.responses["204"]; !exists {
		t.Fatalf("expected default 204 response to remain configured")
	}
}

func TestGeneratorFixtures(t *testing.T) {
	t.Parallel()

	cases := []string{
		"document_basic.json",
		"document_nested.json",
		"document_empty_collections.json",
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

			if !reflect.DeepEqual(fx.Expect.Document, got) {
				t.Fatalf("schema mismatch\nwant: %#v\ngot:  %#v", fx.Expect.Document, got)
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
	if schema["openapi"] == "" {
		t.Fatalf("expected openapi version, got %v", schema["openapi"])
	}
	paths, ok := schema["paths"].(map[string]any)
	if !ok || len(paths) == 0 {
		t.Fatalf("expected paths definition, got %v", schema["paths"])
	}
}

type fixture struct {
	Snapshot map[string]any `json:"snapshot"`
	Expect   struct {
		Document map[string]any `json:"document"`
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
