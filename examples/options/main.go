package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	opts "github.com/goliatone/opts"
	openapi "github.com/goliatone/opts/schema/openapi"
)

// ServerConfig holds server settings and demonstrates OpenAPI metadata tags.
type ServerConfig struct {
	Port         int    `json:"port" default:"8080" minimum:"1" maximum:"65535" formgen:"label=HTTP Port" relationship:"type=belongsTo,target=Environment"`
	Host         string `json:"host" default:"localhost" minLength:"3" formgen:"label=Host,placeholder=app.local"`
	ReadTimeout  int    `json:"readTimeout" default:"30" minimum:"0" formgen:"hint=Seconds until read timeout"`
	WriteTimeout int    `json:"writeTimeout" default:"30" minimum:"0"`
	Enabled      bool   `json:"enabled" default:"true" formgen:"label=Enable server"`
}

// Validate ensures the server configuration is valid.
func (s ServerConfig) Validate() error {
	if !s.Enabled {
		return errors.New("server is disabled")
	}
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("port %d out of range (1-65535)", s.Port)
	}
	if s.Host == "" {
		return errors.New("host must not be empty")
	}
	if s.ReadTimeout < 0 {
		return errors.New("readTimeout must be non-negative")
	}
	if s.WriteTimeout < 0 {
		return errors.New("writeTimeout must be non-negative")
	}
	return nil
}

// DatabaseConfig represents database configuration with constraint metadata.
type DatabaseConfig struct {
	Host       string `json:"host" default:"localhost" formgen:"label=Database Host"`
	Port       int    `json:"port" default:"5432" minimum:"1024" maximum:"65535"`
	MaxRetries int    `json:"maxRetries" default:"3" minimum:"0" maximum:"10" formgen:"hint=Automatic retry attempts"`
	SSL        bool   `json:"ssl" default:"true"`
}

// FeatureFlag defines a reusable component for feature flags.
type FeatureFlag struct {
	Key     string `json:"key" formgen:"label=Flag Key"`
	Enabled bool   `json:"enabled" default:"false"`
}

// AppConfig holds application configuration with nested structs and arrays.
type AppConfig struct {
	Name        string         `json:"name" default:"options-service" formgen:"label=Service Name"`
	Environment string         `json:"environment" default:"production" enum:"production,staging,development" formgen:"widget=select"`
	Server      ServerConfig   `json:"server"`
	Database    DatabaseConfig `json:"database"`
	Flags       []FeatureFlag  `json:"flags"`
}

// Example demonstrating defaults, validation, layering, schema inspection, and OpenAPI export.
func main() {
	fmt.Println("=== go-options Example ===")

	// 1. Defaults
	fmt.Println("1. Applying defaults")
	defaults := ServerConfig{
		Port:         8080,
		Host:         "localhost",
		ReadTimeout:  30,
		WriteTimeout: 30,
		Enabled:      true,
	}
	current := opts.ApplyDefaults(ServerConfig{}, defaults)
	fmt.Printf("   Applied: %+v\n\n", current)

	// 2. Validation
	fmt.Println("2. Validation")
	validConfig := ServerConfig{
		Port:         8080,
		Host:         "localhost",
		ReadTimeout:  30,
		WriteTimeout: 30,
		Enabled:      true,
	}
	validWrapper, err := opts.Load(validConfig)
	if err != nil {
		fmt.Printf("   Valid config failed: %v\n", err)
	} else {
		fmt.Println("   Valid config passed validation")
	}

	invalidConfig := ServerConfig{
		Port:         8080,
		Host:         "localhost",
		ReadTimeout:  30,
		WriteTimeout: 30,
		Enabled:      false, // Will fail validation
	}
	_, err = opts.Load(invalidConfig)
	if err != nil {
		fmt.Printf("   Invalid config failed as expected: %v\n\n", err)
	}

	// 3. Layering
	fmt.Println("3. Layering configuration")
	baseConfig := map[string]any{
		"timeout": 30,
		"retries": 3,
		"debug":   true,
	}
	prodOverrides := map[string]any{
		"timeout": 60,
		"debug":   false,
	}
	userOverrides := map[string]any{
		"retries": 5,
	}

	wrapper := opts.New(baseConfig)
	merged := wrapper.LayerWith(userOverrides, prodOverrides)

	timeout, _ := merged.Get("timeout")
	retries, _ := merged.Get("retries")
	debug, _ := merged.Get("debug")
	fmt.Printf("   Merged config: timeout=%v, retries=%v, debug=%v\n\n", timeout, retries, debug)

	// 4. Schema inspection (descriptor format)
	fmt.Println("4. Schema inspection (descriptor format)")
	schemaWrapper := opts.New(AppConfig{})
	schemaDoc := schemaWrapper.MustSchema()
	fields, ok := schemaDoc.Document.([]opts.FieldDescriptor)
	if !ok {
		log.Fatalf("unexpected schema document type %T", schemaDoc.Document)
	}

	fmt.Printf("   Total fields: %d\n", len(fields))
	fmt.Println("   Fields:")
	for _, field := range fields {
		fmt.Printf("     %s => %s\n", field.Path, field.Type)
	}
	fmt.Println()

	// 5. OpenAPI schema export with metadata-rich output
	fmt.Println("5. OpenAPI schema export")
	openAPIWrapper := opts.New(
		AppConfig{},
		openapi.Option(
			openapi.WithInfo("Options Service Schema", "1.0.0", openapi.WithInfoDescription("Sample configuration for go-options")),
			openapi.WithOperation("/config", "POST", "post:/config", openapi.WithOperationSummary("Submit configuration")),
			openapi.WithContentType("application/json"),
		),
	)
	openAPIDoc, err := openAPIWrapper.Schema()
	if err != nil {
		log.Fatalf("failed to generate OpenAPI schema: %v", err)
	}
	openAPIJSON, err := json.MarshalIndent(openAPIDoc.Document, "   ", "  ")
	if err != nil {
		log.Fatalf("failed to marshal OpenAPI schema: %v", err)
	}
	fmt.Println("   OpenAPI schema:")
	fmt.Printf("%s\n\n", openAPIJSON)

	// 6. Dynamic access with Get/Set
	fmt.Println("6. Dynamic access (Get/Set)")
	dynamicWrapper := opts.New(map[string]any{
		"api": map[string]any{
			"endpoint": "https://api.example.com",
			"version":  "v1",
		},
	})

	endpoint, _ := dynamicWrapper.Get("api.endpoint")
	version, _ := dynamicWrapper.Get("api.version")
	fmt.Printf("   Get api.endpoint: %v\n", endpoint)
	fmt.Printf("   Get api.version: %v\n", version)

	err = dynamicWrapper.Set("api.timeout", 30)
	if err != nil {
		log.Fatalf("Failed to set api.timeout: %v", err)
	}
	timeout, _ = dynamicWrapper.Get("api.timeout")
	fmt.Printf("   Set api.timeout: %v\n\n", timeout)

	// 7. Rule evaluation
	fmt.Println("7. Rule evaluation")
	evalWrapper := opts.New(map[string]any{
		"features": map[string]any{
			"newUI":     true,
			"darkMode":  false,
			"analytics": true,
		},
	})

	result, err := evalWrapper.Evaluate("features.newUI && features.analytics")
	if err != nil {
		log.Fatalf("Failed to evaluate: %v", err)
	}
	fmt.Printf("   Expression: features.newUI && features.analytics\n")
	fmt.Printf("   Result: %v\n", result.Value)

	// Verify validWrapper is used
	serverPort, _ := validWrapper.Get("Port")
	fmt.Printf("\n8. Validated config in use\n")
	fmt.Printf("   Server port: %v\n", serverPort)
}
