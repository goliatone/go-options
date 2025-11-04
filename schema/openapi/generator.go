package openapi

import opts "github.com/goliatone/opts"

type generator struct {
	config generatorConfig
}

// NewGenerator constructs an OpenAPI-compatible schema generator.
func NewGenerator(options ...GeneratorOption) opts.SchemaGenerator {
	cfg := defaultGeneratorConfig()
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	return generator{config: cfg}
}

// Option returns an opts.Option that wires the OpenAPI schema generator into an Options wrapper.
func Option(options ...GeneratorOption) opts.Option {
	return opts.WithSchemaGenerator(NewGenerator(options...))
}

func (g generator) Generate(value any) (opts.SchemaDocument, error) {
	node, err := buildSchemaGraph(value)
	if err != nil {
		return opts.SchemaDocument{}, err
	}
	registry := newComponentRegistry()
	builder := newOpenAPIDocumentBuilder(g.config, registry, node)
	document, err := builder.build()
	if err != nil {
		return opts.SchemaDocument{}, err
	}
	return opts.SchemaDocument{
		Format:   opts.SchemaFormatOpenAPI,
		Document: document,
	}, nil
}
