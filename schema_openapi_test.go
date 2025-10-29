package opts_test

import (
	"testing"

	opts "github.com/goliatone/opts"
	openapi "github.com/goliatone/opts/schema/openapi"
)

func TestOpenAPIGeneratorIntegration(t *testing.T) {
	wrapper := opts.New(map[string]any{
		"enabled": true,
		"name":    "service",
	}, openapi.Option())

	doc, err := wrapper.Schema()
	if err != nil {
		t.Fatalf("Schema returned error: %v", err)
	}
	if doc.Format != opts.SchemaFormatOpenAPI {
		t.Fatalf("expected format %q, got %q", opts.SchemaFormatOpenAPI, doc.Format)
	}
	schema, ok := doc.Document.(map[string]any)
	if !ok {
		t.Fatalf("expected schema map, got %T", doc.Document)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map, got %T", schema["properties"])
	}
	if _, exists := properties["enabled"]; !exists {
		t.Fatalf("expected properties to include enabled")
	}
	if _, exists := properties["name"]; !exists {
		t.Fatalf("expected properties to include name")
	}
}
