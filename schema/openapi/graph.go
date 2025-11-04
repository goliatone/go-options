package openapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

type schemaNode struct {
	Type              string
	Format            string
	Properties        map[string]*schemaNode
	Required          []string
	Items             *schemaNode
	Enum              []any
	Default           any
	Minimum           *float64
	Maximum           *float64
	ExclusiveMinimum  *float64
	ExclusiveMaximum  *float64
	MinLength         *int
	MaxLength         *int
	Pattern           string
	formgen           map[string]string
	relationships     map[string]string
	additionalMapping map[string]any
}

func newObjectNode() *schemaNode {
	return &schemaNode{
		Type:       "object",
		Properties: map[string]*schemaNode{},
	}
}

func (n *schemaNode) baseMap() map[string]any {
	result := map[string]any{}
	if n.Type != "" {
		result["type"] = n.Type
	}
	if n.Format != "" {
		result["format"] = n.Format
	}
	if n.Default != nil {
		result["default"] = n.Default
	}
	if len(n.Enum) > 0 {
		result["enum"] = n.Enum
	}
	if n.Minimum != nil {
		result["minimum"] = *n.Minimum
	}
	if n.Maximum != nil {
		result["maximum"] = *n.Maximum
	}
	if n.ExclusiveMinimum != nil {
		result["exclusiveMinimum"] = *n.ExclusiveMinimum
	}
	if n.ExclusiveMaximum != nil {
		result["exclusiveMaximum"] = *n.ExclusiveMaximum
	}
	if n.MinLength != nil {
		result["minLength"] = *n.MinLength
	}
	if n.MaxLength != nil {
		result["maxLength"] = *n.MaxLength
	}
	if n.Pattern != "" {
		result["pattern"] = n.Pattern
	}
	return result
}

func (n *schemaNode) inlineOpenAPI() map[string]any {
	result := n.baseMap()

	if len(n.Properties) > 0 || n.Type == "object" {
		props := make(map[string]any, len(n.Properties))
		names := make([]string, 0, len(n.Properties))
		for name := range n.Properties {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			props[name] = n.Properties[name].inlineOpenAPI()
		}
		result["properties"] = props
	}

	if len(n.Required) > 0 {
		names := append([]string{}, n.Required...)
		sort.Strings(names)
		result["required"] = names
	}

	if n.Items != nil {
		result["items"] = n.Items.inlineOpenAPI()
	}

	if len(n.formgen) > 0 {
		result["x-formgen"] = orderedStringMap(n.formgen)
	}

	if len(n.relationships) > 0 {
		result["x-relationships"] = orderedStringMap(n.relationships)
	}

	if len(n.additionalMapping) > 0 {
		keys := make([]string, 0, len(n.additionalMapping))
		for key := range n.additionalMapping {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			result[key] = n.additionalMapping[key]
		}
	}

	return result
}

func (n *schemaNode) ensureFormgen() map[string]string {
	if n.formgen == nil {
		n.formgen = map[string]string{}
	}
	return n.formgen
}

func (n *schemaNode) ensureRelationships() map[string]string {
	if n.relationships == nil {
		n.relationships = map[string]string{}
	}
	return n.relationships
}

func (n *schemaNode) Digest() string {
	payload := n.inlineOpenAPI()
	data, err := json.Marshal(payload)
	if err != nil {
		// json.Marshal should never fail for the constructed payload; fall back to
		// an empty digest to avoid panics.
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type schemaBuilder struct {
	visited map[reflect.Type]bool
}

func newSchemaBuilder() *schemaBuilder {
	return &schemaBuilder{
		visited: map[reflect.Type]bool{},
	}
}

func buildSchemaGraph(value any) (*schemaNode, error) {
	builder := newSchemaBuilder()
	rv := reflect.ValueOf(value)
	var rt reflect.Type
	if rv.IsValid() {
		rt = rv.Type()
	}
	node, err := builder.build(rv, rt)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return newObjectNode(), nil
	}
	if node.Type == "" {
		node.Type = "object"
	}
	if node.Type == "object" && node.Properties == nil {
		node.Properties = map[string]*schemaNode{}
	}
	return node, nil
}

func (b *schemaBuilder) build(rv reflect.Value, rt reflect.Type) (*schemaNode, error) {
	if rt == nil {
		if rv.IsValid() {
			rt = rv.Type()
		} else {
			return newObjectNode(), nil
		}
	}

	for rt.Kind() == reflect.Pointer {
		if rv.IsValid() {
			if rv.IsNil() {
				rv = reflect.Value{}
			} else {
				rv = rv.Elem()
			}
		}
		rt = rt.Elem()
	}

	if rt.Kind() == reflect.Interface {
		if rv.IsValid() && !rv.IsNil() {
			return b.build(rv.Elem(), rv.Elem().Type())
		}
		return newObjectNode(), nil
	}

	if rt == reflect.TypeOf(time.Time{}) {
		return &schemaNode{
			Type:   "string",
			Format: "date-time",
		}, nil
	}

	switch rt.Kind() {
	case reflect.Bool:
		return &schemaNode{Type: "boolean"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return &schemaNode{Type: "integer"}, nil
	case reflect.Float32, reflect.Float64:
		return &schemaNode{Type: "number"}, nil
	case reflect.String:
		return &schemaNode{Type: "string"}, nil
	case reflect.Struct:
		return b.buildStruct(rv, rt)
	case reflect.Map:
		return b.buildMap(rv, rt)
	case reflect.Slice, reflect.Array:
		if rt.Kind() == reflect.Slice && rt.Elem().Kind() == reflect.Uint8 {
			return &schemaNode{
				Type:   "string",
				Format: "byte",
			}, nil
		}
		return b.buildSlice(rv, rt)
	default:
		return &schemaNode{
			Type:   "string",
			Format: fmt.Sprintf("go:%s", rt.String()),
		}, nil
	}
}

func (b *schemaBuilder) buildStruct(rv reflect.Value, rt reflect.Type) (*schemaNode, error) {
	if b.visited[rt] {
		return newObjectNode(), nil
	}
	b.visited[rt] = true
	defer delete(b.visited, rt)

	if !rv.IsValid() {
		rv = reflect.Zero(rt)
	}

	node := newObjectNode()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}

		name, omitEmpty, skip := parseJSONName(field)
		if skip {
			continue
		}
		fieldValue := reflect.Value{}
		if rv.IsValid() {
			fieldValue = rv.Field(i)
		}

		child, err := b.build(fieldValue, field.Type)
		if err != nil {
			return nil, err
		}

		if err := applyFieldMetadata(child, field); err != nil {
			return nil, err
		}

		if node.Properties == nil {
			node.Properties = map[string]*schemaNode{}
		}
		node.Properties[name] = child

		if isFieldRequired(field, omitEmpty) {
			node.Required = append(node.Required, name)
		}
	}

	return node, nil
}

func (b *schemaBuilder) buildMap(rv reflect.Value, rt reflect.Type) (*schemaNode, error) {
	if rt.Key().Kind() != reflect.String {
		return nil, fmt.Errorf("openapi: map key type %s unsupported", rt.Key())
	}

	node := newObjectNode()
	if !rv.IsValid() || rv.Len() == 0 {
		return node, nil
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

	for _, name := range names {
		value := rv.MapIndex(reflect.ValueOf(name))
		child, err := b.build(value, value.Type())
		if err != nil {
			return nil, err
		}
		node.Properties[name] = child
	}

	return node, nil
}

func (b *schemaBuilder) buildSlice(rv reflect.Value, rt reflect.Type) (*schemaNode, error) {
	node := &schemaNode{
		Type: "array",
	}
	elemType := rt.Elem()
	var elemValue reflect.Value
	if rv.IsValid() && rv.Len() > 0 {
		elemValue = rv.Index(0)
	} else if elemType.Kind() != reflect.Invalid {
		elemValue = reflect.Zero(elemType)
	}

	child, err := b.build(elemValue, elemType)
	if err != nil {
		return nil, err
	}
	node.Items = child
	return node, nil
}

func parseJSONName(field reflect.StructField) (name string, omitEmpty bool, skip bool) {
	tag := field.Tag.Get("json")
	if tag == "" {
		return field.Name, false, false
	}

	segments := strings.Split(tag, ",")
	if segments[0] == "-" {
		return "", false, true
	}

	name = segments[0]
	if name == "" {
		name = field.Name
	}
	for _, segment := range segments[1:] {
		if segment == "omitempty" {
			omitEmpty = true
		}
	}
	return name, omitEmpty, false
}

func isFieldRequired(field reflect.StructField, omitEmpty bool) bool {
	if omitEmpty {
		return false
	}
	ft := field.Type
	for ft.Kind() == reflect.Pointer {
		return false
	}
	return true
}

func applyFieldMetadata(node *schemaNode, field reflect.StructField) error {
	baseType := field.Type
	for baseType.Kind() == reflect.Pointer {
		baseType = baseType.Elem()
	}

	if format := field.Tag.Get("format"); format != "" {
		node.Format = format
	}

	if def := field.Tag.Get("default"); def != "" {
		value, err := parseScalar(baseType, def)
		if err != nil {
			return fmt.Errorf("openapi: parse default for field %s: %w", field.Name, err)
		}
		node.Default = value
	}

	if enum := field.Tag.Get("enum"); enum != "" {
		values, err := parseEnum(baseType, enum)
		if err != nil {
			return fmt.Errorf("openapi: parse enum for field %s: %w", field.Name, err)
		}
		node.Enum = values
	}

	if err := applyNumericConstraints(node, baseType, field); err != nil {
		return err
	}

	if err := applyStringConstraints(node, baseType, field); err != nil {
		return err
	}

	if tag := field.Tag.Get("formgen"); tag != "" {
		values := parseKeyValueTag(tag)
		if len(values) > 0 {
			formgen := node.ensureFormgen()
			for key, value := range values {
				formgen[key] = value
			}
		}
	}

	if tag := field.Tag.Get("relationship"); tag != "" {
		values := parseKeyValueTag(tag)
		if len(values) > 0 {
			meta := node.ensureRelationships()
			for key, value := range values {
				meta[key] = value
			}
		}
	}

	return nil
}

func applyNumericConstraints(node *schemaNode, baseType reflect.Type, field reflect.StructField) error {
	if !isNumericKind(baseType.Kind()) {
		return nil
	}

	assign := func(target **float64, raw string) error {
		if raw == "" {
			return nil
		}
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return err
		}
		*target = &value
		return nil
	}

	if err := assign(&node.Minimum, field.Tag.Get("minimum")); err != nil {
		return fmt.Errorf("openapi: parse minimum for field %s: %w", field.Name, err)
	}
	if err := assign(&node.Maximum, field.Tag.Get("maximum")); err != nil {
		return fmt.Errorf("openapi: parse maximum for field %s: %w", field.Name, err)
	}
	if err := assign(&node.ExclusiveMinimum, field.Tag.Get("exclusiveMinimum")); err != nil {
		return fmt.Errorf("openapi: parse exclusiveMinimum for field %s: %w", field.Name, err)
	}
	if err := assign(&node.ExclusiveMaximum, field.Tag.Get("exclusiveMaximum")); err != nil {
		return fmt.Errorf("openapi: parse exclusiveMaximum for field %s: %w", field.Name, err)
	}

	return nil
}

func applyStringConstraints(node *schemaNode, baseType reflect.Type, field reflect.StructField) error {
	if baseType.Kind() != reflect.String {
		return nil
	}

	assign := func(target **int, raw string) error {
		if raw == "" {
			return nil
		}
		value, err := strconv.Atoi(raw)
		if err != nil {
			return err
		}
		*target = &value
		return nil
	}

	if err := assign(&node.MinLength, field.Tag.Get("minLength")); err != nil {
		return fmt.Errorf("openapi: parse minLength for field %s: %w", field.Name, err)
	}
	if err := assign(&node.MaxLength, field.Tag.Get("maxLength")); err != nil {
		return fmt.Errorf("openapi: parse maxLength for field %s: %w", field.Name, err)
	}
	if pattern := field.Tag.Get("pattern"); pattern != "" {
		node.Pattern = pattern
	}

	return nil
}

func parseScalar(t reflect.Type, raw string) (any, error) {
	switch t.Kind() {
	case reflect.Bool:
		return strconv.ParseBool(raw)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(raw, 10, t.Bits())
		if err != nil {
			return nil, err
		}
		return value, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		value, err := strconv.ParseUint(raw, 10, t.Bits())
		if err != nil {
			return nil, err
		}
		return value, nil
	case reflect.Float32, reflect.Float64:
		return strconv.ParseFloat(raw, t.Bits())
	case reflect.String:
		return raw, nil
	default:
		// Fallback to string representation
		return raw, nil
	}
}

func parseEnum(t reflect.Type, raw string) ([]any, error) {
	parts := strings.Split(raw, ",")
	values := make([]any, 0, len(parts))
	base := t
	for base.Kind() == reflect.Pointer {
		base = base.Elem()
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		value, err := parseScalar(base, part)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func parseKeyValueTag(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	values := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, found := strings.Cut(part, "=")
		if !found {
			key = part
			value = ""
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		values[key] = value
	}
	return values
}

func isNumericKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func orderedStringMap(values map[string]string) map[string]any {
	out := make(map[string]any, len(values))
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out[key] = values[key]
	}
	return out
}
