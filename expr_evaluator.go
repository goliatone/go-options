package opts

import (
	"fmt"

	exprlang "github.com/expr-lang/expr"
	exprvm "github.com/expr-lang/expr/vm"
)

// ExprEvaluatorOption configures an expr evaluator instance.
type ExprEvaluatorOption func(*exprEvaluator)

// ExprWithProgramCache wires a ProgramCache into the expr evaluator.
func ExprWithProgramCache(cache ProgramCache) ExprEvaluatorOption {
	return func(e *exprEvaluator) {
		e.cache = cache
	}
}

// ExprWithFunctionRegistry wires a FunctionRegistry into the expr evaluator.
func ExprWithFunctionRegistry(registry *FunctionRegistry) ExprEvaluatorOption {
	return func(e *exprEvaluator) {
		if registry == nil {
			return
		}
		e.registry = registry.Clone()
	}
}

// exprEvaluator executes rule expressions using github.com/expr-lang/expr.
type exprEvaluator struct {
	cache    ProgramCache
	registry *FunctionRegistry
}

// NewExprEvaluator constructs an Evaluator backed by expr-lang/expr.
func NewExprEvaluator(opts ...ExprEvaluatorOption) Evaluator {
	e := &exprEvaluator{}
	for _, opt := range opts {
		if opt != nil {
			opt(e)
		}
	}
	return e
}

// Evaluate compiles and runs expression against ctx.Snapshot.
func (e *exprEvaluator) Evaluate(ctx RuleContext, expression string) (any, error) {
	if expression == "" {
		return nil, wrapEvaluatorError("expr", fmt.Errorf("expression must not be empty"))
	}
	ctx = ctx.withDefaultNow().withDefaultMaps()
	env := e.environment(ctx)
	if e.cache == nil {
		result, err := exprlang.Eval(expression, env)
		if err != nil {
			return nil, wrapEvaluationError("expr", expression, ctx.scopeLabel(), err)
		}
		return result, nil
	}
	program, err := e.loadOrCompile(expression)
	if err != nil {
		return nil, err
	}
	result, err := exprlang.Run(program, env)
	if err != nil {
		return nil, wrapEvaluationError("expr", expression, ctx.scopeLabel(), err)
	}
	return result, nil
}

// Compile returns a compiled rule that evaluates expression per invocation.
func (e *exprEvaluator) Compile(expression string, _ ...CompileOption) (CompiledRule, error) {
	if expression == "" {
		return nil, wrapEvaluatorError("expr", fmt.Errorf("expression must not be empty"))
	}
	program, err := e.loadOrCompile(expression)
	if err != nil {
		return nil, err
	}
	return &exprCompiledRule{
		evaluator:  e,
		program:    program,
		expression: expression,
	}, nil
}

func (e *exprEvaluator) loadOrCompile(expression string) (*exprvm.Program, error) {
	if e.cache != nil {
		if cached, ok := e.cache.Get(expression); ok {
			if program, ok := cached.(*exprvm.Program); ok {
				return program, nil
			}
		}
	}
	options := []exprlang.Option{
		exprlang.Env(map[string]any{}),
		exprlang.AllowUndefinedVariables(),
	}
	for _, name := range e.registryNames() {
		fn := e.registryFunction(name)
		options = append(options, exprlang.Function(name, fn))
	}
	program, err := exprlang.Compile(expression, options...)
	if err != nil {
		return nil, wrapEvaluationError("expr", expression, "", err)
	}
	if e.cache != nil {
		e.cache.Set(expression, program)
	}
	return program, nil
}

type exprCompiledRule struct {
	evaluator  *exprEvaluator
	program    *exprvm.Program
	expression string
}

func (r *exprCompiledRule) Evaluate(ctx RuleContext) (any, error) {
	if r.evaluator == nil {
		return nil, wrapEvaluatorError("expr", fmt.Errorf("compiled rule missing evaluator"))
	}
	ctx = ctx.withDefaultNow().withDefaultMaps()
	if r.program == nil {
		return r.evaluator.Evaluate(ctx, r.expression)
	}
	env := r.evaluator.environment(ctx)
	result, err := exprlang.Run(r.program, env)
	if err != nil {
		return nil, wrapEvaluationError("expr", r.expression, ctx.scopeLabel(), err)
	}
	return result, nil
}

func (e *exprEvaluator) environment(ctx RuleContext) map[string]any {
	env := map[string]any{
		"now":      ctx.timestamp(),
		"args":     ctx.Args,
		"metadata": ctx.Metadata,
	}
	if binding := ctx.scopeBinding(); binding != nil {
		env["scope"] = binding
	}
	if snapshot, ok := ctx.Snapshot.(map[string]any); ok {
		for key, value := range snapshot {
			env[key] = value
		}
	}
	if e.registry != nil {
		env["call"] = func(name string, arguments ...any) (any, error) {
			return e.registry.Call(name, arguments...)
		}
		for _, name := range e.registry.Names() {
			fn := name
			env[fn] = func(arguments ...any) (any, error) {
				return e.registry.Call(fn, arguments...)
			}
		}
	}
	return env
}

func (e *exprEvaluator) registryNames() []string {
	if e == nil || e.registry == nil {
		return nil
	}
	return e.registry.Names()
}

func (e *exprEvaluator) registryFunction(name string) func(...any) (any, error) {
	if e == nil || e.registry == nil {
		return nil
	}
	return func(arguments ...any) (any, error) {
		return e.registry.Call(name, arguments...)
	}
}
