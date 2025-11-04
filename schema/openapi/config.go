package openapi

import (
	"strings"
)

type generatorConfig struct {
	openAPIVersion string
	info           openapiInfo
	operation      operationConfig
	contentType    string
	responses      map[string]responseConfig
	rootComponent  string
}

type openapiInfo struct {
	Title       string
	Version     string
	Description string
}

type operationConfig struct {
	Path        string
	Method      string
	OperationID string
	Summary     string
}

type responseConfig struct {
	Description string
}

func defaultGeneratorConfig() generatorConfig {
	return generatorConfig{
		openAPIVersion: "3.0.3",
		info: openapiInfo{
			Title:   "Options Schema",
			Version: "1.0.0",
		},
		operation: operationConfig{
			Path:        "/config",
			Method:      "post",
			OperationID: "post:/config",
		},
		contentType: "application/json",
		responses: map[string]responseConfig{
			"204": {
				Description: "OK",
			},
		},
	}
}

// GeneratorOption configures the OpenAPI generator behaviour.
type GeneratorOption func(*generatorConfig)

// WithOpenAPIVersion overrides the OpenAPI version string (default: 3.0.3).
func WithOpenAPIVersion(version string) GeneratorOption {
	return func(cfg *generatorConfig) {
		if version == "" {
			return
		}
		cfg.openAPIVersion = version
	}
}

// InfoOption configures optional fields on the OpenAPI info section.
type InfoOption func(*openapiInfo)

// WithInfoDescription sets the optional description field for the info section.
func WithInfoDescription(description string) InfoOption {
	return func(info *openapiInfo) {
		info.Description = description
	}
}

// WithInfo configures the OpenAPI info block (title/version are required by
// go-formgen). Empty strings retain the existing values.
func WithInfo(title, version string, opts ...InfoOption) GeneratorOption {
	return func(cfg *generatorConfig) {
		if title != "" {
			cfg.info.Title = title
		}
		if version != "" {
			cfg.info.Version = version
		}
		for _, opt := range opts {
			if opt != nil {
				opt(&cfg.info)
			}
		}
	}
}

// OperationOption configures optional operation metadata.
type OperationOption func(*operationConfig)

// WithOperationSummary attaches a summary to the configured operation.
func WithOperationSummary(summary string) OperationOption {
	return func(operation *operationConfig) {
		operation.Summary = summary
	}
}

// WithOperation configures the primary path, method, and operationId used for
// the generated OpenAPI document. Empty inputs retain the defaults.
func WithOperation(path, method, operationID string, opts ...OperationOption) GeneratorOption {
	return func(cfg *generatorConfig) {
		if path != "" {
			cfg.operation.Path = path
		}
		if method != "" {
			cfg.operation.Method = strings.ToLower(method)
		}
		if operationID != "" {
			cfg.operation.OperationID = operationID
		}
		for _, opt := range opts {
			if opt != nil {
				opt(&cfg.operation)
			}
		}
	}
}

// WithContentType sets the preferred content type for the request body.
func WithContentType(contentType string) GeneratorOption {
	return func(cfg *generatorConfig) {
		if contentType == "" {
			return
		}
		cfg.contentType = contentType
	}
}

// ResponseOption configures additional response metadata.
type ResponseOption func(*responseConfig)

// WithResponse registers or overrides a response template for the provided status code.
func WithResponse(status, description string, opts ...ResponseOption) GeneratorOption {
	return func(cfg *generatorConfig) {
		if status == "" {
			return
		}
		if cfg.responses == nil {
			cfg.responses = map[string]responseConfig{}
		}
		resp := cfg.responses[status]
		if description != "" {
			resp.Description = description
		}
		for _, opt := range opts {
			if opt != nil {
				opt(&resp)
			}
		}
		cfg.responses[status] = resp
	}
}

// WithRootComponent forces the root schema to be published under components with the provided name.
func WithRootComponent(name string) GeneratorOption {
	return func(cfg *generatorConfig) {
		cfg.rootComponent = name
	}
}
