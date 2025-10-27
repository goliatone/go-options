package opts

import (
	"fmt"
	"sort"
	"strings"
)

// Schema represents read-only descriptors for the wrapped value.
type Schema struct {
	Fields []FieldDescriptor
}

// FieldDescriptor describes a path and the inferred type.
type FieldDescriptor struct {
	Path string
	Type string
}

func deriveSchema(value any, prefix string) []FieldDescriptor {
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
			fields = append(fields, deriveSchema(typed[key], nextPrefix)...)
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
