package opts

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Schema represents read-only descriptors for the wrapped value.
type Schema struct {
	Fields []FieldDescriptor
}

// FieldDescriptor describes a path and its inferred type.
type FieldDescriptor struct {
	Path string
	Type string
}

func deriveSchema(value any, prefix string) []FieldDescriptor {
	if value == nil {
		if prefix == "" {
			return nil
		}
		return []FieldDescriptor{{
			Path: prefix,
			Type: "nil",
		}}
	}

	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return nil
	}

	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			if prefix == "" {
				return nil
			}
			return []FieldDescriptor{{
				Path: prefix,
				Type: typeLabel(rv.Type()),
			}}
		}
		return deriveSchema(rv.Elem().Interface(), prefix)
	}

	switch rv.Kind() {
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			if prefix == "" {
				return nil
			}
			return []FieldDescriptor{{
				Path: prefix,
				Type: rv.Type().String(),
			}}
		}
		if rv.Len() == 0 {
			if prefix == "" {
				return nil
			}
			return []FieldDescriptor{{
				Path: prefix,
				Type: fmt.Sprintf("map[string]%s", typeLabel(rv.Type().Elem())),
			}}
		}
		keys := rv.MapKeys()
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})
		var fields []FieldDescriptor
		for _, key := range keys {
			nextPrefix := joinPath(prefix, key.String())
			fields = append(fields, deriveSchema(rv.MapIndex(key).Interface(), nextPrefix)...)
		}
		return fields
	case reflect.Struct:
		var entries []struct {
			path  string
			value any
		}
		for i := 0; i < rv.NumField(); i++ {
			sf := rv.Type().Field(i)
			if !sf.IsExported() {
				continue
			}
			name := sf.Name
			if tag := sf.Tag.Get("json"); tag != "" && tag != "-" {
				tagName := strings.Split(tag, ",")[0]
				if tagName != "" {
					name = tagName
				}
			}
			field := rv.Field(i)
			if !field.CanInterface() {
				continue
			}
			entries = append(entries, struct {
				path  string
				value any
			}{
				path:  joinPath(prefix, name),
				value: field.Interface(),
			})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].path < entries[j].path
		})
		var fields []FieldDescriptor
		for _, entry := range entries {
			fields = append(fields, deriveSchema(entry.value, entry.path)...)
		}
		return fields
	case reflect.Slice, reflect.Array:
		elementType := typeLabel(rv.Type().Elem())
		if prefix == "" {
			return nil
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
			Type: typeLabel(rv.Type()),
		}}
	}
}

func typeLabel(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Bool:
		return "bool"
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "int"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return "uint"
	case reflect.Float32, reflect.Float64:
		return "float"
	case reflect.Slice, reflect.Array:
		return "[]" + typeLabel(t.Elem())
	case reflect.Map:
		if t.Key().Kind() == reflect.String {
			return fmt.Sprintf("map[string]%s", typeLabel(t.Elem()))
		}
		return t.String()
	case reflect.Pointer:
		return "*" + typeLabel(t.Elem())
	case reflect.Interface:
		return "any"
	default:
		return t.String()
	}
}

func joinPath(prefix, segment string) string {
	if prefix == "" {
		return segment
	}
	return strings.Join([]string{prefix, segment}, ".")
}

func sortFields(fields []FieldDescriptor) {
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Path < fields[j].Path
	})
}
