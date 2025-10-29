package main

import (
	"fmt"
	"log"

	opts "github.com/goliatone/opts"
	layering "github.com/goliatone/opts/layering"
)

// Example: Multi-environment web service configuration
// This demonstrates a realistic scenario where configuration is layered:
// 1. Base defaults (development environment)
// 2. Production overrides (applied when deploying to prod)
// 3. Runtime overrides (from feature flags or admin console)

func main() {
	// Layer 1: Base defaults for development environment
	baseDefaults := map[string]any{
		"Server": map[string]any{
			"Port":         8080,
			"Host":         "localhost",
			"ReadTimeout":  30,
			"WriteTimeout": 30,
		},
		"Database": map[string]any{
			"Host":            "localhost",
			"Port":            5432,
			"Name":            "myapp_dev",
			"MaxConnections":  10,
			"ConnectionRetry": true,
		},
		"Features": map[string]any{
			"RateLimiting": map[string]any{
				"Enabled":        false,
				"RequestsPerMin": 100,
			},
			"Authentication": map[string]any{
				"OAuth":   false,
				"JWT":     true,
				"TokenTTL": 3600,
			},
			"Caching": map[string]any{
				"Enabled": false,
				"TTL":     300,
			},
		},
		"Logging": map[string]any{
			"Level":  "debug",
			"Format": "json",
		},
	}

	// Layer 2: Production environment overrides
	productionOverrides := map[string]any{
		"Server": map[string]any{
			"Host":         "0.0.0.0",
			"ReadTimeout":  60,
			"WriteTimeout": 60,
		},
		"Database": map[string]any{
			"Host":           "db.production.internal",
			"Name":           "myapp_prod",
			"MaxConnections": 100,
		},
		"Features": map[string]any{
			"RateLimiting": map[string]any{
				"Enabled":        true,
				"RequestsPerMin": 1000,
			},
			"Caching": map[string]any{
				"Enabled": true,
				"TTL":     600,
			},
		},
		"Logging": map[string]any{
			"Level": "info",
		},
	}

	// Layer 3: Runtime overrides (feature flags, emergency toggles)
	runtimeOverrides := map[string]any{
		"Features": map[string]any{
			"RateLimiting": map[string]any{
				"RequestsPerMin": 500, // Reduced due to high load
			},
			"Authentication": map[string]any{
				"OAuth": true, // New feature enabled
			},
		},
	}

	// Merge layers: runtime > production > defaults
	config := layering.MergeLayers(runtimeOverrides, productionOverrides, baseDefaults)
	wrapper := opts.New(config, opts.WithEvaluator(opts.NewExprEvaluator()))

	// Use case 1: Check if rate limiting should be applied
	rateLimitResp, err := wrapper.Evaluate("Features.RateLimiting.Enabled")
	if err != nil {
		log.Fatalf("Failed to evaluate rate limiting: %v", err)
	}
	rateLimitEnabled := rateLimitResp.Value.(bool)

	// Use case 2: Get rate limit value for middleware configuration
	requestsPerMin, err := wrapper.Get("Features.RateLimiting.RequestsPerMin")
	if err != nil {
		log.Fatalf("Failed to get rate limit value: %v", err)
	}

	// Use case 3: Check authentication strategy
	oauthEnabled, _ := wrapper.Get("Features.Authentication.OAuth")
	jwtEnabled, _ := wrapper.Get("Features.Authentication.JWT")

	// Use case 4: Validate caching configuration
	cachingResp, err := wrapper.Evaluate("Features.Caching.Enabled && Features.Caching.TTL > 0")
	if err != nil {
		log.Fatalf("Failed to evaluate caching config: %v", err)
	}
	cachingValid := cachingResp.Value.(bool)

	// Use case 5: Database connection string construction
	dbHost, _ := wrapper.Get("Database.Host")
	dbPort, _ := wrapper.Get("Database.Port")
	dbName, _ := wrapper.Get("Database.Name")
	dbMaxConns, _ := wrapper.Get("Database.MaxConnections")

	// Display the final merged configuration
	fmt.Println("=== Web Service Configuration ===")
	fmt.Printf("\nServer:\n")
	serverHost, _ := wrapper.Get("Server.Host")
	serverPort, _ := wrapper.Get("Server.Port")
	fmt.Printf("  Address: %s:%v\n", serverHost, serverPort)

	fmt.Printf("\nDatabase:\n")
	fmt.Printf("  Connection: %s:%v/%s\n", dbHost, dbPort, dbName)
	fmt.Printf("  Max Connections: %v\n", dbMaxConns)

	fmt.Printf("\nFeatures:\n")
	fmt.Printf("  Rate Limiting: %v (%v req/min)\n", rateLimitEnabled, requestsPerMin)
	fmt.Printf("  Authentication: OAuth=%v, JWT=%v\n", oauthEnabled, jwtEnabled)
	fmt.Printf("  Caching: Valid=%v\n", cachingValid)

	loggingLevel, _ := wrapper.Get("Logging.Level")
	fmt.Printf("\nLogging Level: %s\n", loggingLevel)

	// Demonstrate schema inspection for debugging/documentation
	schema := wrapper.Schema()
	fmt.Printf("\nTotal configuration fields: %d\n", len(schema.Fields))

	// Example: Find all feature flags
	fmt.Println("\nFeature flags:")
	for _, field := range schema.Fields {
		if len(field.Path) >= 17 && field.Path[:9] == "Features." &&
			len(field.Path) >= 7 && field.Path[len(field.Path)-7:] == "Enabled" {
			value, _ := wrapper.Get(field.Path)
			fmt.Printf("  %s: %v\n", field.Path, value)
		}
	}
}
