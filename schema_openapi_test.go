package opts_test

import (
	"sync"
	"testing"

	opts "github.com/goliatone/go-options"
	openapi "github.com/goliatone/go-options/schema/openapi"
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

	if version, _ := schema["openapi"].(string); version == "" {
		t.Fatalf("expected openapi version string, got %v", schema["openapi"])
	}
	info, ok := schema["info"].(map[string]any)
	if !ok {
		t.Fatalf("expected info section, got %T", schema["info"])
	}
	if title, _ := info["title"].(string); title == "" {
		t.Fatalf("expected info.title value, got %v", info["title"])
	}
	if schemaVersion, _ := info["version"].(string); schemaVersion == "" {
		t.Fatalf("expected info.version value, got %v", info["version"])
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
	if required, _ := requestBody["required"].(bool); !required {
		t.Fatalf("expected requestBody.required to be true, got %v", requestBody["required"])
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
	responses, ok := operation["responses"].(map[string]any)
	if !ok || len(responses) == 0 {
		t.Fatalf("expected responses map, got %v", operation["responses"])
	}
}

func TestOpenAPIGeneratorNilValue(t *testing.T) {
	var snapshot map[string]any
	wrapper := opts.New(snapshot, openapi.Option())

	doc, err := wrapper.Schema()
	if err != nil {
		t.Fatalf("Schema returned error: %v", err)
	}
	if doc.Format != opts.SchemaFormatOpenAPI {
		t.Fatalf("expected format %q, got %q", opts.SchemaFormatOpenAPI, doc.Format)
	}
	payload, ok := doc.Document.(map[string]any)
	if !ok {
		t.Fatalf("expected schema map, got %T", doc.Document)
	}
	if version, _ := payload["openapi"].(string); version == "" {
		t.Fatalf("expected openapi version, got %v", payload["openapi"])
	}
	paths, ok := payload["paths"].(map[string]any)
	if !ok || len(paths) == 0 {
		t.Fatalf("expected non-empty paths, got %v", payload["paths"])
	}
}

func TestOpenAPIGeneratorConcurrentSchema(t *testing.T) {
	wrapper := opts.New(map[string]any{
		"enabled": true,
		"name":    "service",
	}, openapi.Option())

	const goroutines = 16
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			doc, err := wrapper.Schema()
			if err != nil {
				t.Errorf("Schema returned error: %v", err)
				return
			}
			if doc.Document == nil {
				t.Errorf("expected schema document payload")
			}
		}()
	}
	wg.Wait()
}
