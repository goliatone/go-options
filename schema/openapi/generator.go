package openapi

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	opts "github.com/goliatone/opts"
)

type generator struct{}

// NewGenerator constructs an OpenAPI-compatible schema generator.
func NewGenerator() opts.SchemaGenerator {
	return generator{}
}

// Option returns an opts.Option that wires the OpenAPI schema generator into an Options wrapper.
func Option() opts.Option {
	return opts.WithSchemaGenerator(NewGenerator())
}

func (generator) Generate(value any) (opts.SchemaDocument, error) {
	schema, err := buildSchema(reflect.ValueOf(value))
	if err != nil {
		return opts.SchemaDocument{}, err
	}
	if schema == nil {
		schema = map[string]any{"type": "null"}
	}
	return opts.SchemaDocument{
		Format:   opts.SchemaFormatOpenAPI,
		Document: schema,
	}, nil
}

func buildSchema(rv reflect.Value) (map[string]any, error) {
	if !rv.IsValid() {
		return map[string]any{"type": "null"}, nil
	}

	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return map[string]any{"type": "null"}, nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Interface:
		if rv.IsNil() {
			return map[string]any{"type": "null"}, nil
		}
		return buildSchema(rv.Elem())
	case reflect.Bool:
		return map[string]any{"type": "boolean"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return map[string]any{"type": "integer"}, nil
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}, nil
	case reflect.String:
		return map[string]any{"type": "string"}, nil
	case reflect.Struct:
		if rv.Type() == reflect.TypeOf(time.Time{}) {
			return map[string]any{
				"type":   "string",
				"format": "date-time",
			}, nil
		}
		return schemaForStruct(rv)
	case reflect.Map:
		return schemaForMap(rv)
	case reflect.Slice, reflect.Array:
		return schemaForSlice(rv)
	default:
		return map[string]any{
			"type":   "string",
			"format": fmt.Sprintf("go:%s", rv.Type().String()),
		}, nil
	}
}

func schemaForMap(rv reflect.Value) (map[string]any, error) {
	if rv.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("openapi: map key type %s unsupported", rv.Type().Key())
	}

	keys := rv.MapKeys()
	names := make([]string, 0, len(keys))
	for _, key := range keys {
		if key.Kind() != reflect.String {
			return nil, fmt.Errorf("openapi: map key kind %s unsupported", key.Kind())
		}
		names = append(names, key.String())
	}
	sort.Strings(names)

	properties := make(map[string]any, len(names))
	for _, name := range names {
		child, err := buildSchema(rv.MapIndex(reflect.ValueOf(name)))
		if err != nil {
			return nil, err
		}
		properties[name] = child
	}
	return map[string]any{
		"type":       "object",
		"properties": properties,
	}, nil
}

func schemaForStruct(rv reflect.Value) (map[string]any, error) {
	rt := rv.Type()
	properties := map[string]any{}
	names := make([]string, 0, rv.NumField())

	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}

		name := field.Name
		if tag := field.Tag.Get("json"); tag != "" {
			tagName := strings.Split(tag, ",")[0]
			if tagName == "-" {
				continue
			}
			if tagName != "" {
				name = tagName
			}
		}
		if name == "" {
			continue
		}

		child, err := buildSchema(rv.Field(i))
		if err != nil {
			return nil, err
		}
		properties[name] = child
		names = append(names, name)
	}

	sort.Strings(names)
	if len(properties) == 0 {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}, nil
	}

	ordered := make(map[string]any, len(properties))
	for _, name := range names {
		ordered[name] = properties[name]
	}
	return map[string]any{
		"type":       "object",
		"properties": ordered,
	}, nil
}

func schemaForSlice(rv reflect.Value) (map[string]any, error) {
	if rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Uint8 {
		return map[string]any{
			"type":   "string",
			"format": "byte",
		}, nil
	}

	length := rv.Len()
	var itemSchema map[string]any
	var err error
	if length > 0 {
		itemSchema, err = buildSchema(rv.Index(0))
		if err != nil {
			return nil, err
		}
	} else {
		itemSchema = map[string]any{}
	}
	return map[string]any{
		"type":  "array",
		"items": itemSchema,
	}, nil
}
