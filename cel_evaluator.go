package opts

import (
	"fmt"

	celgo "github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// CELEvaluatorOption configures the CEL evaluator.
type CELEvaluatorOption func(*celEvaluator)

// CELWithProgramCache wires a ProgramCache into the CEL evaluator.
func CELWithProgramCache(cache ProgramCache) CELEvaluatorOption {
	return func(e *celEvaluator) {
		e.cache = cache
	}
}

// CELWithFunctionRegistry wires a FunctionRegistry into the CEL evaluator.
func CELWithFunctionRegistry(registry *FunctionRegistry) CELEvaluatorOption {
	return func(e *celEvaluator) {
		if registry == nil {
			return
		}
		e.registry = registry.Clone()
	}
}

type celProgram struct {
	env     *celgo.Env
	program celgo.Program
}

type celEvaluator struct {
	cache    ProgramCache
	registry *FunctionRegistry
}

// NewCELEvaluator constructs an Evaluator backed by cel-go.
func NewCELEvaluator(opts ...CELEvaluatorOption) Evaluator {
	e := &celEvaluator{}
	for _, opt := range opts {
		if opt != nil {
			opt(e)
		}
	}
	return e
}

func (e *celEvaluator) Evaluate(ctx RuleContext, expression string) (any, error) {
	if expression == "" {
		return nil, wrapEvaluatorError("cel", fmt.Errorf("expression must not be empty"))
	}
	ctx = ctx.withDefaultNow().withDefaultMaps()
	snapshot := snapshotAsMap(ctx.Snapshot)
	program, err := e.loadOrCompile(expression, snapshot)
	if err != nil {
		return nil, err
	}
	out, _, err := program.program.Eval(e.activation(ctx, snapshot))
	if err != nil {
		return nil, wrapEvaluatorError("cel", err)
	}
	return out.Value(), nil
}

func (e *celEvaluator) Compile(expression string, _ ...CompileOption) (CompiledRule, error) {
	if expression == "" {
		return nil, wrapEvaluatorError("cel", fmt.Errorf("expression must not be empty"))
	}
	return &celCompiledRule{
		evaluator:  e,
		expression: expression,
	}, nil
}

func (e *celEvaluator) loadOrCompile(expression string, snapshot map[string]any) (*celProgram, error) {
	if snapshot == nil {
		snapshot = map[string]any{}
	}
	if e.cache != nil {
		if cached, ok := e.cache.Get(expression); ok {
			if program, ok := cached.(*celProgram); ok {
				return program, nil
			}
		}
	}

	env, err := e.buildEnv(snapshot)
	if err != nil {
		return nil, wrapEvaluatorError("cel", err)
	}
	ast, issues := env.Parse(expression)
	if issues != nil && issues.Err() != nil {
		return nil, wrapEvaluatorError("cel", issues.Err())
	}
	checked, issues := env.Check(ast)
	if issues != nil && issues.Err() != nil {
		return nil, wrapEvaluatorError("cel", issues.Err())
	}
	prg, err := env.Program(checked)
	if err != nil {
		return nil, wrapEvaluatorError("cel", err)
	}

	bundle := &celProgram{
		env:     env,
		program: prg,
	}
	if e.cache != nil {
		e.cache.Set(expression, bundle)
	}
	return bundle, nil
}

func (e *celEvaluator) buildEnv(snapshot map[string]any) (*celgo.Env, error) {
	opts := []celgo.EnvOption{
		celgo.Variable("now", celgo.TimestampType),
		celgo.Variable("args", celgo.DynType),
		celgo.Variable("metadata", celgo.DynType),
	}
	if e.registry != nil {
		opts = append(opts, celgo.Function("call", e.buildCallOverloads()...))
		for _, name := range e.registry.Names() {
			opts = append(opts, celgo.Function(name, e.buildDirectOverloads(name)...))
		}
	}
	for key := range snapshot {
		opts = append(opts, celgo.Variable(key, celgo.DynType))
	}
	return celgo.NewEnv(opts...)
}

func (e *celEvaluator) activation(ctx RuleContext, snapshot map[string]any) map[string]any {
	activation := map[string]any{
		"now":      ctx.timestamp(),
		"args":     ctx.Args,
		"metadata": ctx.Metadata,
	}
	for key, value := range snapshot {
		activation[key] = value
	}
	if e.registry != nil {
		activation["call"] = func(name string, arguments ...any) (any, error) {
			return e.registry.Call(name, arguments...)
		}
	}
	return activation
}

type celCompiledRule struct {
	evaluator  *celEvaluator
	expression string
}

func (r *celCompiledRule) Evaluate(ctx RuleContext) (any, error) {
	if r.evaluator == nil {
		return nil, wrapEvaluatorError("cel", fmt.Errorf("compiled rule missing evaluator"))
	}
	ctx = ctx.withDefaultNow().withDefaultMaps()
	snapshot := snapshotAsMap(ctx.Snapshot)
	program, err := r.evaluator.loadOrCompile(r.expression, snapshot)
	if err != nil {
		return nil, err
	}
	out, _, err := program.program.Eval(r.evaluator.activation(ctx, snapshot))
	if err != nil {
		return nil, wrapEvaluatorError("cel", err)
	}
	return out.Value(), nil
}

func snapshotAsMap(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func (e *celEvaluator) callBinding() func(...ref.Val) ref.Val {
	return func(values ...ref.Val) ref.Val {
		if e.registry == nil {
			return types.NewErr("opts: function registry not configured")
		}
		if len(values) == 0 {
			return types.NewErr("opts: call requires function name")
		}
		name, ok := values[0].Value().(string)
		if !ok {
			return types.NewErr("opts: call name must be string")
		}
		args := make([]any, 0, len(values)-1)
		for _, val := range values[1:] {
			args = append(args, val.Value())
		}
		result, err := e.registry.Call(name, args...)
		if err != nil {
			return types.NewErr(err.Error())
		}
		if result == nil {
			return types.NullValue
		}
		return types.DefaultTypeAdapter.NativeToValue(result)
	}
}

func (e *celEvaluator) directBinding(name string) func(...ref.Val) ref.Val {
	return func(values ...ref.Val) ref.Val {
		if e.registry == nil {
			return types.NewErr("opts: function registry not configured")
		}
		args := make([]any, 0, len(values))
		for _, val := range values {
			args = append(args, val.Value())
		}
		result, err := e.registry.Call(name, args...)
		if err != nil {
			return types.NewErr(err.Error())
		}
		if result == nil {
			return types.NullValue
		}
		return types.DefaultTypeAdapter.NativeToValue(result)
	}
}

func (e *celEvaluator) buildCallOverloads() []celgo.FunctionOpt {
	const maxArity = 6
	fnOpts := make([]celgo.FunctionOpt, 0, maxArity)
	for arity := 1; arity <= maxArity; arity++ {
		argTypes := make([]*celgo.Type, 1, arity)
		argTypes[0] = celgo.StringType
		for i := 1; i < arity; i++ {
			argTypes = append(argTypes, celgo.DynType)
		}
		fnOpts = append(fnOpts, celgo.Overload(
			fmt.Sprintf("call_string_dyn_%d", arity-1),
			argTypes,
			celgo.DynType,
			celgo.FunctionBinding(e.callBinding()),
		))
	}
	return fnOpts
}

func (e *celEvaluator) buildDirectOverloads(name string) []celgo.FunctionOpt {
	const maxArity = 6
	fnOpts := make([]celgo.FunctionOpt, 0, maxArity+1)
	fnOpts = append(fnOpts, celgo.Overload(
		fmt.Sprintf("%s_dyn_0", name),
		[]*celgo.Type{},
		celgo.DynType,
		celgo.FunctionBinding(e.directBinding(name)),
	))
	for arity := 1; arity <= maxArity; arity++ {
		argTypes := make([]*celgo.Type, 0, arity)
		for i := 0; i < arity; i++ {
			argTypes = append(argTypes, celgo.DynType)
		}
		fnOpts = append(fnOpts, celgo.Overload(
			fmt.Sprintf("%s_dyn_%d", name, arity),
			argTypes,
			celgo.DynType,
			celgo.FunctionBinding(e.directBinding(name)),
		))
	}
	return fnOpts
}
