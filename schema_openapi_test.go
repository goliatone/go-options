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
	paths, ok := schema["paths"].(map[string]any)
	if !ok {
		t.Fatalf("expected paths map, got %T", schema["paths"])
	}
	pathItem, ok := paths["/config"].(map[string]any)
	if !ok {
		t.Fatalf("expected /config path map, got %T", paths["/config"])
	}
	operation, ok := pathItem["post"].(map[string]any)
	if !ok {
		t.Fatalf("expected post operation map, got %T", pathItem["post"])
	}
	requestBody, ok := operation["requestBody"].(map[string]any)
	if !ok {
		t.Fatalf("expected requestBody map, got %T", operation["requestBody"])
	}
	content, ok := requestBody["content"].(map[string]any)
	if !ok {
		t.Fatalf("expected content map, got %T", requestBody["content"])
	}
	media, ok := content["application/json"].(map[string]any)
	if !ok {
		t.Fatalf("expected application/json content, got %T", content["application/json"])
	}
	bodySchema, ok := media["schema"].(map[string]any)
	if !ok {
		t.Fatalf("expected schema map, got %T", media["schema"])
	}
	properties, ok := bodySchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map, got %T", bodySchema["properties"])
	}
	if _, exists := properties["enabled"]; !exists {
		t.Fatalf("expected properties to include enabled")
	}
	if _, exists := properties["name"]; !exists {
		t.Fatalf("expected properties to include name")
	}
}
