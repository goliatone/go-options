package opts

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Function represents a callable registered against evaluators.
type Function func(args ...any) (any, error)

// FunctionRegistry stores custom functions keyed by name.
type FunctionRegistry struct {
	mu        sync.RWMutex
	functions map[string]Function
}

// NewFunctionRegistry constructs an empty registry.
func NewFunctionRegistry() *FunctionRegistry {
	return &FunctionRegistry{
		functions: make(map[string]Function),
	}
}

// Register stores fn under name guarding against duplicates.
func (r *FunctionRegistry) Register(name string, fn Function) error {
	if fn == nil {
		return fmt.Errorf("opts: function %q is nil", name)
	}
	if name == "" {
		return fmt.Errorf("opts: function name must not be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.functions == nil {
		r.functions = make(map[string]Function)
	}
	key := strings.ToLower(name)
	if _, exists := r.functions[key]; exists {
		return fmt.Errorf("opts: function %q already registered", name)
	}
	r.functions[key] = fn
	return nil
}

// Clone returns a shallow copy of the registry.
func (r *FunctionRegistry) Clone() *FunctionRegistry {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	clone := &FunctionRegistry{
		functions: make(map[string]Function, len(r.functions)),
	}
	for name, fn := range r.functions {
		clone.functions[name] = fn
	}
	return clone
}

// Call executes the function registered for name.
func (r *FunctionRegistry) Call(name string, args ...any) (any, error) {
	if r == nil {
		return nil, fmt.Errorf("opts: function registry is nil")
	}
	r.mu.RLock()
	fn := r.functions[strings.ToLower(name)]
	r.mu.RUnlock()
	if fn == nil {
		return nil, fmt.Errorf("opts: function %q not registered", name)
	}
	return fn(args...)
}

// Names returns registered function names sorted alphabetically.
func (r *FunctionRegistry) Names() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.functions))
	for name := range r.functions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// WithFunctionRegistry configures a wrapper to use registry.
func WithFunctionRegistry(registry *FunctionRegistry) Option {
	return func(cfg *optionsConfig) {
		if registry == nil {
			return
		}
		cfg.functions = registry.Clone()
	}
}

// WithCustomFunction registers fn under name for the wrapper.
func WithCustomFunction(name string, fn Function) Option {
	return func(cfg *optionsConfig) {
		if cfg.functions == nil {
			cfg.functions = NewFunctionRegistry()
		}
		_ = cfg.functions.Register(name, fn)
	}
}
