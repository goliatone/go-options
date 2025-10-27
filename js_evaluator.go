package opts

import (
	"fmt"

	"github.com/dop251/goja"
)

// JSEvaluatorOption configures the JS evaluator instance.
type JSEvaluatorOption func(*jsEvaluator)

// JSWithProgramCache wires a ProgramCache into the JS evaluator.
func JSWithProgramCache(cache ProgramCache) JSEvaluatorOption {
	return func(e *jsEvaluator) {
		e.cache = cache
	}
}

// JSWithFunctionRegistry wires a FunctionRegistry into the JS evaluator.
func JSWithFunctionRegistry(registry *FunctionRegistry) JSEvaluatorOption {
	return func(e *jsEvaluator) {
		if registry == nil {
			return
		}
		e.registry = registry.Clone()
	}
}

type jsEvaluator struct {
	cache    ProgramCache
	registry *FunctionRegistry
}

// NewJSEvaluator constructs an Evaluator backed by goja.
func NewJSEvaluator(opts ...JSEvaluatorOption) Evaluator {
	e := &jsEvaluator{}
	for _, opt := range opts {
		if opt != nil {
			opt(e)
		}
	}
	return e
}

func (e *jsEvaluator) Evaluate(ctx RuleContext, expression string) (any, error) {
	if expression == "" {
		return nil, wrapEvaluatorError("js", fmt.Errorf("expression must not be empty"))
	}
	ctx = ctx.withDefaultNow().withDefaultMaps()
	if e.cache == nil {
		return e.run(ctx, expression, nil)
	}
	program, err := e.loadOrCompile(expression)
	if err != nil {
		return nil, err
	}
	return e.run(ctx, expression, program)
}

func (e *jsEvaluator) Compile(expression string, _ ...CompileOption) (CompiledRule, error) {
	if expression == "" {
		return nil, wrapEvaluatorError("js", fmt.Errorf("expression must not be empty"))
	}
	program, err := e.loadOrCompile(expression)
	if err != nil {
		return nil, err
	}
	return &jsCompiledRule{
		evaluator:  e,
		expression: expression,
		program:    program,
	}, nil
}

func (e *jsEvaluator) loadOrCompile(expression string) (*goja.Program, error) {
	if e.cache != nil {
		if cached, ok := e.cache.Get(expression); ok {
			if program, ok := cached.(*goja.Program); ok {
				return program, nil
			}
		}
	}
	program, err := goja.Compile("", e.wrapExpression(expression), false)
	if err != nil {
		return nil, wrapEvaluatorError("js", err)
	}
	if e.cache != nil {
		e.cache.Set(expression, program)
	}
	return program, nil
}

func (e *jsEvaluator) run(ctx RuleContext, expression string, program *goja.Program) (any, error) {
	vm := goja.New()
	e.injectContext(vm, ctx)
	if program != nil {
		value, err := vm.RunProgram(program)
		if err != nil {
			return nil, wrapEvaluatorError("js", err)
		}
		return value.Export(), nil
	}
	value, err := vm.RunString(e.wrapExpression(expression))
	if err != nil {
		return nil, wrapEvaluatorError("js", err)
	}
	return value.Export(), nil
}

func (e *jsEvaluator) injectContext(vm *goja.Runtime, ctx RuleContext) {
	vm.Set("now", ctx.timestamp())
	vm.Set("args", ctx.Args)
	vm.Set("metadata", ctx.Metadata)
	if snapshot, ok := ctx.Snapshot.(map[string]any); ok {
		for key, value := range snapshot {
			vm.Set(key, value)
		}
	}
	if e.registry != nil {
		vm.Set("call", func(name string, arguments ...any) (any, error) {
			return e.registry.Call(name, arguments...)
		})
		for _, name := range e.registry.Names() {
			fn := name
			vm.Set(fn, func(arguments ...any) (any, error) {
				return e.registry.Call(fn, arguments...)
			})
		}
	}
}

func (e *jsEvaluator) wrapExpression(expression string) string {
	return fmt.Sprintf("(function(){ return (%s); })()", expression)
}

type jsCompiledRule struct {
	evaluator  *jsEvaluator
	expression string
	program    *goja.Program
}

func (r *jsCompiledRule) Evaluate(ctx RuleContext) (any, error) {
	if r.evaluator == nil {
		return nil, wrapEvaluatorError("js", fmt.Errorf("compiled rule missing evaluator"))
	}
	ctx = ctx.withDefaultNow().withDefaultMaps()
	return r.evaluator.run(ctx, r.expression, r.program)
}
