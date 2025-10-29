package opts

import (
	"fmt"
	"sort"
	"strings"
)

// FieldDescriptor describes a path and the inferred type.
type FieldDescriptor struct {
	Path string
	Type string
}

// DefaultSchemaGenerator returns the built-in descriptor-based schema generator.
func DefaultSchemaGenerator() SchemaGenerator {
	return descriptorGenerator{}
}

type descriptorGenerator struct{}

func (descriptorGenerator) Generate(value any) (SchemaDocument, error) {
	descriptors := deriveFieldDescriptors(value, "")
	if descriptors == nil {
		descriptors = []FieldDescriptor{}
	}
	return SchemaDocument{
		Format:   SchemaFormatDescriptors,
		Document: descriptors,
	}, nil
}

func deriveFieldDescriptors(value any, prefix string) []FieldDescriptor {
	if value == nil {
		return nil
	}

	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			return []FieldDescriptor{{
				Path: prefix,
				Type: "map[string]any",
			}}
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var fields []FieldDescriptor
		for _, key := range keys {
			nextPrefix := joinPath(prefix, key)
			fields = append(fields, deriveFieldDescriptors(typed[key], nextPrefix)...)
		}
		return fields
	case []any:
		elementType := "any"
		if len(typed) > 0 {
			elementType = typeName(typed[0])
		}
		return []FieldDescriptor{{
			Path: prefix,
			Type: "[]" + elementType,
		}}
	default:
		if prefix == "" {
			return nil
		}
		return []FieldDescriptor{{
			Path: prefix,
			Type: typeName(typed),
		}}
	}
}

func typeName(value any) string {
	if value == nil {
		return "nil"
	}
	return fmt.Sprintf("%T", value)
}

func joinPath(prefix, segment string) string {
	if prefix == "" {
		return segment
	}
	return strings.Join([]string{prefix, segment}, ".")
}
