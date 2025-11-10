package opts

import (
	"fmt"
	"reflect"
	"sort"
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

// Clone returns a shallow copy of the Options wrapper preserving configuration.
func (o *Options[T]) Clone() *Options[T] {
	if o == nil {
		return nil
	}
	clone := *o
	if len(o.layers) > 0 {
		clone.layers = append([]layerSnapshot(nil), o.layers...)
	}
	return &clone
}

// WithValue returns a cloned wrapper whose Value field is set to value, leaving
// the original untouched.
func (o *Options[T]) WithValue(value T) *Options[T] {
	if o == nil {
		return &Options[T]{Value: value}
	}
	clone := o.Clone()
	clone.Value = value
	return clone
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

// ResolveWithTrace returns the effective value for path along with provenance
// from each scope layer that was inspected.
func (o *Options[T]) ResolveWithTrace(path string) (any, Trace, error) {
	trace := Trace{Path: path}
	if o == nil {
		return nil, trace, fmt.Errorf("opts: nil options wrapper")
	}
	segments, err := splitPath(path)
	if err != nil {
		return nil, trace, err
	}

	layers := o.layerSnapshots()
	if len(layers) == 0 {
		value, err := o.Get(path)
		if err != nil {
			return nil, trace, err
		}
		trace.Layers = []Provenance{{
			Scope: o.cfg.scope.clone(),
			Path:  path,
			Value: value,
			Found: true,
		}}
		return value, trace, nil
	}

	var (
		value    any
		resolved bool
	)
	for _, layer := range layers {
		prov := Provenance{
			Scope:      layer.Scope,
			SnapshotID: layer.SnapshotID,
			Path:       path,
		}
		layerValue, err := navigateSegments(layer.Snapshot, segments)
		if err == nil {
			prov.Found = true
			prov.Value = layerValue
			if !resolved {
				value = layerValue
				resolved = true
			}
		}
		trace.Layers = append(trace.Layers, prov)
	}

	if resolved {
		return value, trace, nil
	}

	finalValue, err := o.Get(path)
	if err != nil {
		return nil, trace, err
	}
	return finalValue, trace, nil
}

// FlattenWithProvenance enumerates every reachable path in the wrapped value
// and reports which scope supplied the effective value.
func (o *Options[T]) FlattenWithProvenance() ([]Provenance, error) {
	if o == nil {
		return nil, fmt.Errorf("opts: nil options wrapper")
	}
	paths := collectPaths(o.Value)
	results := make([]Provenance, 0, len(paths))
	for _, path := range paths {
		_, trace, err := o.ResolveWithTrace(path)
		if err != nil {
			return nil, err
		}
		var recorded bool
		for _, layer := range trace.Layers {
			if layer.Found {
				results = append(results, layer)
				recorded = true
				break
			}
		}
		if !recorded && len(trace.Layers) > 0 {
			results = append(results, trace.Layers[len(trace.Layers)-1])
		}
	}
	return results, nil
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

// Schema returns a schema document for the wrapped value.
func (o *Options[T]) Schema() (SchemaDocument, error) {
	generator := o.schemaGenerator()
	var snapshot any
	if o != nil {
		snapshot = any(o.Value)
	}
	doc, err := generator.Generate(snapshot)
	if err != nil {
		return SchemaDocument{}, err
	}
	if doc.Format == "" {
		doc.Format = SchemaFormatDescriptors
	}
	if doc.Document == nil {
		doc.Document = []FieldDescriptor{}
	}
	if o != nil && o.cfg.scopeSchema {
		if scopes := describeSchemaScopes(o.layerSnapshots()); len(scopes) > 0 {
			doc.Scopes = scopes
		}
	}
	return doc, nil
}

// MustSchema generates a schema document and panics on error.
func (o *Options[T]) MustSchema() SchemaDocument {
	doc, err := o.Schema()
	if err != nil {
		panic(fmt.Sprintf("opts: schema generation failed: %v", err))
	}
	return doc
}

// WithSchemaGenerator returns a cloned wrapper configured with generator.
// Passing a nil generator removes any custom generator and falls back to the default.
func (o *Options[T]) WithSchemaGenerator(generator SchemaGenerator) *Options[T] {
	if o == nil {
		return &Options[T]{cfg: optionsConfig{schemaGenerator: generator}}
	}
	clone := o.Clone()
	clone.cfg.schemaGenerator = generator
	return clone
}

func (o *Options[T]) attachLayers(layers []layerSnapshot) {
	if o == nil {
		return
	}
	if len(layers) == 0 {
		o.layers = nil
		return
	}
	o.layers = append([]layerSnapshot(nil), layers...)
}

func (o *Options[T]) layerSnapshots() []layerSnapshot {
	if o == nil || len(o.layers) == 0 {
		return nil
	}
	out := make([]layerSnapshot, len(o.layers))
	copy(out, o.layers)
	return out
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

func navigateSegments(value any, segments []string) (any, error) {
	var err error
	current := value
	for _, segment := range segments {
		current, err = navigateSegment(current, segment)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

func collectPaths(value any) []string {
	paths := map[string]struct{}{}
	seen := map[uintptr]struct{}{}
	flattenValue(reflect.ValueOf(value), "", paths, seen)
	keys := make([]string, 0, len(paths))
	for path := range paths {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	return keys
}

func flattenValue(rv reflect.Value, prefix string, paths map[string]struct{}, seen map[uintptr]struct{}) {
	if !rv.IsValid() {
		if prefix != "" {
			paths[prefix] = struct{}{}
		}
		return
	}

	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.Kind() == reflect.Interface {
			if rv.IsNil() {
				if prefix != "" {
					paths[prefix] = struct{}{}
				}
				return
			}
			rv = rv.Elem()
			continue
		}
		if rv.IsNil() {
			if prefix != "" {
				paths[prefix] = struct{}{}
			}
			return
		}
		ptr := rv.Pointer()
		if _, ok := seen[ptr]; ok {
			return
		}
		seen[ptr] = struct{}{}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Struct:
		for i := 0; i < rv.NumField(); i++ {
			sf := rv.Type().Field(i)
			if !sf.IsExported() {
				continue
			}
			segment := structFieldSegment(sf)
			if segment == "" {
				continue
			}
			flattenValue(rv.Field(i), appendPathSegment(prefix, segment), paths, seen)
		}
	case reflect.Map:
		if rv.IsNil() {
			if prefix != "" {
				paths[prefix] = struct{}{}
			}
			return
		}
		if rv.Type().Key().Kind() != reflect.String {
			if prefix != "" {
				paths[prefix] = struct{}{}
			}
			return
		}
		iter := rv.MapRange()
		for iter.Next() {
			key := iter.Key().String()
			flattenValue(iter.Value(), appendPathSegment(prefix, key), paths, seen)
		}
	case reflect.Slice, reflect.Array:
		if rv.Len() == 0 {
			if prefix != "" {
				paths[prefix] = struct{}{}
			}
			return
		}
		for i := 0; i < rv.Len(); i++ {
			flattenValue(rv.Index(i), appendPathSegment(prefix, strconv.Itoa(i)), paths, seen)
		}
	default:
		if prefix != "" {
			paths[prefix] = struct{}{}
		}
	}
}

func structFieldSegment(sf reflect.StructField) string {
	if tag := sf.Tag.Get("json"); tag != "" && tag != "-" {
		parts := strings.Split(tag, ",")
		if parts[0] != "" {
			return parts[0]
		}
	}
	return sf.Name
}

func appendPathSegment(prefix, segment string) string {
	if prefix == "" {
		return segment
	}
	if segment == "" {
		return prefix
	}
	return prefix + "." + segment
}

func describeSchemaScopes(layers []layerSnapshot) []SchemaScope {
	if len(layers) == 0 {
		return nil
	}
	out := make([]SchemaScope, len(layers))
	for i, layer := range layers {
		out[i] = SchemaScope{
			Name:       layer.Scope.Name,
			Label:      layer.Scope.Label,
			Priority:   layer.Scope.Priority,
			SnapshotID: layer.SnapshotID,
		}
		if len(layer.Scope.Metadata) > 0 {
			out[i].Metadata = copyMetadata(layer.Scope.Metadata)
		}
	}
	return out
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
