package openapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
		"document_minimal.json",
		"document_nested.json",
		"document_arrays.json",
		"document_components.json",
		"document_custom_extensions.json",
	}

	for _, name := range cases {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fx := loadFixture(t, name)
			input := fx.value(t)

			generator := NewGenerator()
			doc, err := generator.Generate(input)
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
			assertJSONEqual(t, fx.Expect.Document, got)

			if err := validateDocument(got); err != nil {
				t.Fatalf("document %s failed validation: %v", name, err)
			}
		})
	}
}

func TestGeneratorNil(t *testing.T) {
	t.Parallel()

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
	if err := validateDocument(schema); err != nil {
		t.Fatalf("nil snapshot produced invalid document: %v", err)
	}
}

func TestGeneratorConcurrentAccess(t *testing.T) {
	t.Parallel()

	generator := NewGenerator()
	input := map[string]any{
		"service": map[string]any{
			"name":    "api",
			"enabled": true,
		},
	}

	const goroutines = 16
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			doc, err := generator.Generate(input)
			if err != nil {
				t.Errorf("Generate returned error: %v", err)
				return
			}
			if doc.Document == nil {
				t.Errorf("expected document payload")
			}
		}()
	}
	wg.Wait()
}

type fixture struct {
	Sample   string         `json:"sample"`
	Snapshot map[string]any `json:"snapshot"`
	Expect   struct {
		Document map[string]any `json:"document"`
	} `json:"expect"`
}

func (fx fixture) value(t *testing.T) any {
	t.Helper()

	switch {
	case fx.Sample != "":
		value, err := buildFixtureSample(fx.Sample)
		if err != nil {
			t.Fatalf("build fixture sample %q: %v", fx.Sample, err)
		}
		return value
	case fx.Snapshot != nil:
		return fx.Snapshot
	default:
		return nil
	}
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

func assertJSONEqual(t *testing.T, want, got map[string]any) {
	t.Helper()

	wantBytes := mustMarshal(t, want)
	gotBytes := mustMarshal(t, got)

	if !bytes.Equal(wantBytes, gotBytes) {
		t.Fatalf("schema mismatch\nwant: %s\ngot:  %s", wantBytes, gotBytes)
	}
}

func mustMarshal(t *testing.T, value any) []byte {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return raw
}

type fixtureCredentials struct {
	Username string `json:"username" default:"admin" minLength:"3" maxLength:"32" formgen:"label=Username,placeholder=Enter username"`
	Password string `json:"password,omitempty" minLength:"12" formgen:"widget=password"`
}

type fixtureServiceConfig struct {
	Name        string               `json:"name" default:"api" formgen:"label=Service Name,placeholder=Enter name"`
	TimeoutSecs int                  `json:"timeoutSecs" minimum:"1" maximum:"120" default:"30" formgen:"hint=Seconds until timeout" relationship:"type=dependsOn,target=Service"`
	Mode        string               `json:"mode,omitempty" enum:"active,passive" formgen:"widget=select" relationship:"type=belongsTo,target=Mode"`
	Credentials fixtureCredentials   `json:"credentials"`
	Replicas    []fixtureCredentials `json:"replicas"`
}

func buildFixtureSample(name string) (any, error) {
	switch name {
	case "document_minimal":
		return map[string]any{
			"enabled":   true,
			"name":      "service",
			"retries":   3,
			"threshold": 0.75,
		}, nil
	case "document_nested":
		return map[string]any{
			"features": map[string]any{
				"flags": []any{
					map[string]any{"name": "beta", "enabled": true},
					map[string]any{"name": "darkMode", "enabled": false},
				},
			},
			"server": map[string]any{
				"hosts": []any{"us-east-1", "us-west-2"},
				"port":  8080,
			},
		}, nil
	case "document_arrays":
		return map[string]any{
			"ids": []any{1, 2, 3},
			"matrix": []any{
				[]any{1, 2},
				[]any{3, 4},
			},
			"services": []any{
				map[string]any{"name": "api", "enabled": true},
				map[string]any{"name": "worker", "enabled": false},
			},
		}, nil
	case "document_components":
		primary := map[string]any{"host": "primary.local", "port": 8080}
		secondary := map[string]any{"host": "secondary.local", "port": 8080}
		return map[string]any{
			"primary":   primary,
			"secondary": secondary,
			"replicas": []any{
				map[string]any{"host": "replica-a.local", "port": 8080},
				map[string]any{"host": "replica-b.local", "port": 8080},
			},
		}, nil
	case "document_custom_extensions":
		return fixtureServiceConfig{
			Name:        "api",
			TimeoutSecs: 45,
			Mode:        "active",
			Credentials: fixtureCredentials{
				Username: "admin",
			},
			Replicas: []fixtureCredentials{
				{Username: "worker-a"},
				{Username: "worker-b"},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unknown sample %q", name)
	}
}
