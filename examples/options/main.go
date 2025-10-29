package main

import (
	"errors"
	"fmt"
	"log"

	opts "github.com/goliatone/opts"
)

// ServerConfig holds server settings.
type ServerConfig struct {
	Port         int    `json:"port"`
	Host         string `json:"host"`
	ReadTimeout  int    `json:"readTimeout"`
	WriteTimeout int    `json:"writeTimeout"`
	Enabled      bool   `json:"enabled"`
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

// AppConfig holds application configuration.
type AppConfig struct {
	Server   ServerConfig      `json:"server"`
	Database map[string]any    `json:"database"`
	Features map[string]any    `json:"features"`
	Logging  map[string]string `json:"logging"`
}

// Example demonstrating defaults, validation, layering, and schema inspection.
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

	// 4. Schema inspection
	fmt.Println("4. Schema inspection")
	configMap := map[string]any{
		"server": map[string]any{
			"host": "localhost",
			"port": 8080,
		},
		"database": map[string]any{
			"host":       "localhost",
			"port":       5432,
			"maxRetries": 3,
			"ssl":        true,
		},
		"features": map[string]any{
			"enabled": true,
			"flags":   []any{"feature1", "feature2"},
		},
	}
	schemaWrapper := opts.New(configMap)
	schema := schemaWrapper.Schema()

	fmt.Printf("   Total fields: %d\n", len(schema.Fields))
	fmt.Println("   Fields:")
	for _, field := range schema.Fields {
		fmt.Printf("     %s => %s\n", field.Path, field.Type)
	}
	fmt.Println()

	// 5. Dynamic access with Get/Set
	fmt.Println("5. Dynamic access (Get/Set)")
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

	// 6. Rule evaluation
	fmt.Println("6. Rule evaluation")
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
	fmt.Printf("\n7. Validated config in use\n")
	fmt.Printf("   Server port: %v\n", serverPort)
}
