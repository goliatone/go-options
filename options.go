package opts

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// New constructs an Options wrapper around the provided value.
func New[T any](value T, opts ...Option) *Options[T] {
	cfg := applyOptions(opts)
	return &Options[T]{
		Value: value,
		cfg:   cfg,
	}
}

// Load constructs an Options wrapper and runs validation when supported by the
// underlying type.
func Load[T any](value T, opts ...Option) (*Options[T], error) {
	wrapper := New(value, opts...)
	if err := validateValue(wrapper.Value); err != nil {
		return nil, err
	}
	return wrapper, nil
}

// ApplyDefaults returns value if it is already populated, otherwise it falls
// back to defaults.
func ApplyDefaults[T any](value T, defaults T) T {
	if isZero(value) {
		return defaults
	}
	return value
}

// WithEvaluator configures an evaluator on the Options wrapper.
func WithEvaluator(e Evaluator) Option {
	return func(cfg *optionsConfig) {
		cfg.evaluator = e
	}
}

// Validate invokes the Validate method on the wrapped value when present.
func (o *Options[T]) Validate() error {
	return validateValue(o.Value)
}

func validateValue[T any](value T) error {
	if v, ok := any(value).(interface{ Validate() error }); ok {
		return v.Validate()
	}
	if rv := reflect.ValueOf(value); rv.Kind() != reflect.Pointer && rv.CanAddr() {
		if v, ok := rv.Addr().Interface().(interface{ Validate() error }); ok {
			return v.Validate()
		}
	}
	return nil
}

func isZero[T any](value T) bool {
	return reflect.ValueOf(value).IsZero()
}

// Get retrieves the value located at path using dot notation.
func (o *Options[T]) Get(path string) (any, error) {
	segments, err := splitPath(path)
	if err != nil {
		return nil, err
	}
	current := any(o.Value)
	for _, segment := range segments {
		current, err = navigateSegment(current, segment)
		if err != nil {
			return nil, fmt.Errorf("opts: path %q cannot traverse segment %q: %w", path, segment, err)
		}
	}
	return current, nil
}

// Set stores value at the path creating intermediate maps as needed.
func (o *Options[T]) Set(path string, value any) error {
	segments, err := splitPath(path)
	if err != nil {
		return err
	}
	if len(segments) == 0 {
		return fmt.Errorf("opts: path must not be empty")
	}
	var current any = o.Value
	for i, segment := range segments[:len(segments)-1] {
		m, ok := current.(map[string]any)
		if !ok {
			return fmt.Errorf("opts: path %q cannot traverse segment %q", path, segments[i])
		}
		next, exists := m[segment]
		if !exists {
			next = map[string]any{}
			m[segment] = next
		}
		current = next
	}
	lastMap, ok := current.(map[string]any)
	if !ok {
		return fmt.Errorf("opts: path %q cannot assign segment %q", path, segments[len(segments)-1])
	}
	lastMap[segments[len(segments)-1]] = value
	return nil
}

// Schema returns a read-only descriptor of the wrapped value.
func (o *Options[T]) Schema() Schema {
	fields := deriveSchema(any(o.Value), "")
	return Schema{Fields: fields}
}

func splitPath(path string) ([]string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("opts: path must not be empty")
	}
	segments := strings.Split(path, ".")
	for _, segment := range segments {
		if segment == "" {
			return nil, fmt.Errorf("opts: path %q contains empty segment", path)
		}
	}
	return segments, nil
}

func navigateSegment(value any, segment string) (any, error) {
	if value == nil {
		return nil, fmt.Errorf("encountered nil value")
	}

	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, fmt.Errorf("encountered nil pointer")
		}
		rv = rv.Elem()
		value = rv.Interface()
	}

	switch rv.Kind() {
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map key type %s unsupported", rv.Type().Key())
		}
		elem := rv.MapIndex(reflect.ValueOf(segment))
		if !elem.IsValid() {
			return nil, fmt.Errorf("segment not found")
		}
		return elem.Interface(), nil
	case reflect.Struct:
		field, ok := structFieldByName(rv, segment)
		if !ok {
			return nil, fmt.Errorf("field not found")
		}
		return field.Interface(), nil
	case reflect.Slice, reflect.Array:
		index, err := strconv.Atoi(segment)
		if err != nil {
			return nil, fmt.Errorf("expected numeric index: %v", err)
		}
		if index < 0 || index >= rv.Len() {
			return nil, fmt.Errorf("index %d out of bounds", index)
		}
		return rv.Index(index).Interface(), nil
	default:
		return nil, fmt.Errorf("type %T does not support navigation", value)
	}
}

func structFieldByName(rv reflect.Value, segment string) (reflect.Value, bool) {
	if !rv.IsValid() {
		return reflect.Value{}, false
	}

	if sf, ok := rv.Type().FieldByName(segment); ok && sf.IsExported() {
		field := rv.FieldByIndex(sf.Index)
		if field.CanInterface() {
			return field, true
		}
	}

	for i := 0; i < rv.NumField(); i++ {
		sf := rv.Type().Field(i)
		if !sf.IsExported() {
			continue
		}
		tag := sf.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == segment {
			field := rv.Field(i)
			if field.CanInterface() {
				return field, true
			}
		}
	}
	return reflect.Value{}, false
}
