package openapi

import (
	"fmt"
	"regexp"
)

type componentRegistry struct {
	entries   map[string]*componentEntry
	usedNames map[string]struct{}
}

type componentEntry struct {
	name   string
	schema map[string]any
	count  int
	force  bool
}

func newComponentRegistry() *componentRegistry {
	return &componentRegistry{
		entries:   map[string]*componentEntry{},
		usedNames: map[string]struct{}{},
	}
}

func (r *componentRegistry) register(nameHint string, node *schemaNode) string {
	return r.registerInternal(nameHint, node, false)
}

func (r *componentRegistry) forceReference(name string, node *schemaNode) string {
	return r.registerInternal(name, node, true)
}

func (r *componentRegistry) registerInternal(nameHint string, node *schemaNode, force bool) string {
	if node == nil {
		return ""
	}
	digest := node.Digest()
	if digest == "" {
		return ""
	}

	if entry, ok := r.entries[digest]; ok {
		entry.count++
		if force {
			entry.force = true
		}
		if entry.schema == nil && (entry.force || entry.count >= 2) {
			entry.schema = node.inlineOpenAPI()
		}
		if entry.force || entry.count >= 2 {
			return fmt.Sprintf("#/components/schemas/%s", entry.name)
		}
		return ""
	}

	name := r.uniqueName(nameHint)
	r.entries[digest] = &componentEntry{
		name: name,
		schema: func() map[string]any {
			if force {
				return node.inlineOpenAPI()
			}
			return nil
		}(),
		count: 1,
		force: force,
	}
	if force {
		return fmt.Sprintf("#/components/schemas/%s", name)
	}
	return ""
}

func (r *componentRegistry) uniqueName(name string) string {
	safe := sanitizeComponentName(name)
	if safe == "" {
		safe = "Schema"
	}
	if _, exists := r.usedNames[safe]; !exists {
		r.usedNames[safe] = struct{}{}
		return safe
	}
	suffix := 1
	for {
		candidate := fmt.Sprintf("%s%d", safe, suffix)
		if _, exists := r.usedNames[candidate]; !exists {
			r.usedNames[candidate] = struct{}{}
			return candidate
		}
		suffix++
	}
}

func (r *componentRegistry) componentsMap() map[string]any {
	out := make(map[string]any, len(r.entries))
	for _, entry := range r.entries {
		if entry.force || entry.count >= 2 {
			if entry.schema == nil {
				entry.schema = map[string]any{}
			}
			out[entry.name] = entry.schema
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

var componentNameRegexp = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func sanitizeComponentName(name string) string {
	name = componentNameRegexp.ReplaceAllString(name, "_")
	name = trimUnderscores(name)
	if name == "" {
		return ""
	}
	if name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}
	return name
}

func trimUnderscores(input string) string {
	start := 0
	for start < len(input) && input[start] == '_' {
		start++
	}
	end := len(input)
	for end > start && input[end-1] == '_' {
		end--
	}
	return input[start:end]
}
