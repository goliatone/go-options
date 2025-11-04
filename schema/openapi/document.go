package openapi

import (
	"fmt"
	"sort"
	"strings"
)

type openAPIDocumentBuilder struct {
	config    generatorConfig
	registry  *componentRegistry
	rootNode  *schemaNode
	rootRef   string
	inlineDoc map[string]any
}

func newOpenAPIDocumentBuilder(config generatorConfig, registry *componentRegistry, root *schemaNode) *openAPIDocumentBuilder {
	return &openAPIDocumentBuilder{
		config:   config,
		registry: registry,
		rootNode: root,
	}
}

func (b *openAPIDocumentBuilder) build() (map[string]any, error) {
	if b.rootNode == nil {
		return nil, fmt.Errorf("openapi: root schema node cannot be nil")
	}

	if b.config.rootComponent != "" {
		b.rootRef = b.registry.forceReference(b.config.rootComponent, b.rootNode)
		b.registerDescendants(b.config.rootComponent, b.rootNode)
	} else {
		// Inline the root schema but still allow component reuse when referenced.
		b.inlineDoc = b.schemaFor(b.rootNode, "Root")
	}

	document := map[string]any{
		"openapi": b.config.openAPIVersion,
		"info":    b.buildInfo(),
		"paths":   b.buildPaths(),
	}

	if components := b.registry.componentsMap(); components != nil {
		document["components"] = map[string]any{
			"schemas": components,
		}
	}

	if err := validateDocument(document); err != nil {
		return nil, err
	}

	return document, nil
}

func (b *openAPIDocumentBuilder) buildInfo() map[string]any {
	info := map[string]any{
		"title":   b.config.info.Title,
		"version": b.config.info.Version,
	}
	if b.config.info.Description != "" {
		info["description"] = b.config.info.Description
	}
	return info
}

func (b *openAPIDocumentBuilder) buildPaths() map[string]any {
	method := strings.ToLower(b.config.operation.Method)
	if method == "" {
		method = "post"
	}

	content := map[string]any{}
	if b.inlineDoc != nil {
		content[b.config.contentType] = map[string]any{
			"schema": b.inlineDoc,
		}
	} else if b.rootRef != "" {
		content[b.config.contentType] = map[string]any{
			"schema": map[string]any{
				"$ref": b.rootRef,
			},
		}
	}
	if len(content) == 0 {
		content[b.config.contentType] = map[string]any{
			"schema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		}
	}

	responses := make(map[string]any, len(b.config.responses))
	statuses := make([]string, 0, len(b.config.responses))
	for status := range b.config.responses {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	for _, status := range statuses {
		resp := b.config.responses[status]
		responses[status] = map[string]any{
			"description": resp.Description,
		}
	}

	operation := map[string]any{
		"operationId": b.operationID(),
		"requestBody": map[string]any{
			"required": true,
			"content":  content,
		},
		"responses": responses,
	}
	if summary := strings.TrimSpace(b.config.operation.Summary); summary != "" {
		operation["summary"] = summary
	}

	return map[string]any{
		b.config.operation.Path: map[string]any{
			method: operation,
		},
	}
}

func (b *openAPIDocumentBuilder) operationID() string {
	if b.config.operation.OperationID != "" {
		return b.config.operation.OperationID
	}
	method := strings.ToLower(b.config.operation.Method)
	if method == "" {
		method = "post"
	}
	return fmt.Sprintf("%s:%s", method, b.config.operation.Path)
}

func (b *openAPIDocumentBuilder) schemaFor(node *schemaNode, nameHint string) map[string]any {
	if node == nil {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}

	useRegistry := node.Type == "object" || node.Type == "array"
	if useRegistry {
		if ref := b.registry.register(nameHint, node); ref != "" {
			return map[string]any{"$ref": ref}
		}
	}

	result := node.baseMap()

	if len(node.Properties) > 0 || node.Type == "object" {
		props := make(map[string]any, len(node.Properties))
		names := make([]string, 0, len(node.Properties))
		for key := range node.Properties {
			names = append(names, key)
		}
		sort.Strings(names)
		for _, key := range names {
			child := node.Properties[key]
			props[key] = b.schemaFor(child, combineComponentName(nameHint, key))
		}
		result["properties"] = props
	}

	if len(node.Required) > 0 {
		required := append([]string{}, node.Required...)
		sort.Strings(required)
		result["required"] = required
	}

	if node.Items != nil {
		result["items"] = b.schemaFor(node.Items, combineComponentName(nameHint, "item"))
	}

	if len(node.formgen) > 0 {
		result["x-formgen"] = orderedStringMap(node.formgen)
	}

	if len(node.relationships) > 0 {
		result["x-relationships"] = orderedStringMap(node.relationships)
	}

	if len(node.additionalMapping) > 0 {
		keys := make([]string, 0, len(node.additionalMapping))
		for key := range node.additionalMapping {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			result[key] = node.additionalMapping[key]
		}
	}

	return result
}

func (b *openAPIDocumentBuilder) registerDescendants(nameHint string, node *schemaNode) {
	if node == nil {
		return
	}
	if len(node.Properties) > 0 {
		names := make([]string, 0, len(node.Properties))
		for key := range node.Properties {
			names = append(names, key)
		}
		sort.Strings(names)
		for _, key := range names {
			child := node.Properties[key]
			b.schemaFor(child, combineComponentName(nameHint, key))
		}
	}
	if node.Items != nil {
		b.schemaFor(node.Items, combineComponentName(nameHint, "item"))
	}
}

func combineComponentName(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		filtered = append(filtered, part)
	}
	if len(filtered) == 0 {
		return "Schema"
	}
	return strings.Join(filtered, "_")
}

func validateDocument(document map[string]any) error {
	if document == nil {
		return fmt.Errorf("openapi: document cannot be nil")
	}
	openapi, _ := document["openapi"].(string)
	if openapi == "" {
		return fmt.Errorf("openapi: document missing version string")
	}
	info, _ := document["info"].(map[string]any)
	if info == nil {
		return fmt.Errorf("openapi: document missing info section")
	}
	if title, _ := info["title"].(string); title == "" {
		return fmt.Errorf("openapi: info.title must be set")
	}
	if version, _ := info["version"].(string); version == "" {
		return fmt.Errorf("openapi: info.version must be set")
	}
	paths, _ := document["paths"].(map[string]any)
	if len(paths) == 0 {
		return fmt.Errorf("openapi: document must define at least one path")
	}
	for pathKey, pathValue := range paths {
		pathItem, _ := pathValue.(map[string]any)
		if pathItem == nil {
			return fmt.Errorf("openapi: path %q invalid payload", pathKey)
		}
		if len(pathItem) == 0 {
			return fmt.Errorf("openapi: path %q missing operations", pathKey)
		}
		for method, operationValue := range pathItem {
			operation, _ := operationValue.(map[string]any)
			if operation == nil {
				return fmt.Errorf("openapi: operation %s %s invalid payload", method, pathKey)
			}
			if _, ok := operation["operationId"].(string); !ok {
				return fmt.Errorf("openapi: operation %s %s missing operationId", method, pathKey)
			}
			requestBody, _ := operation["requestBody"].(map[string]any)
			if requestBody == nil {
				return fmt.Errorf("openapi: operation %s %s missing requestBody", method, pathKey)
			}
			content, _ := requestBody["content"].(map[string]any)
			if len(content) == 0 {
				return fmt.Errorf("openapi: operation %s %s requestBody missing content", method, pathKey)
			}
			if _, ok := operation["responses"].(map[string]any); !ok {
				return fmt.Errorf("openapi: operation %s %s missing responses", method, pathKey)
			}
		}
	}
	return nil
}
