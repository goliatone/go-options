package opts

import (
	"errors"
	"fmt"
	"time"
)

var ErrNoEvaluator = errors.New("opts: evaluator not configured")

// Evaluate executes expr using the configured evaluator and wraps the result.
func (o *Options[T]) Evaluate(expr string) (Response[any], error) {
	if expr == "" {
		return Response[any]{}, fmt.Errorf("expression must not be empty")
	}
	evaluator, err := o.resolveEvaluator()
	if err != nil {
		return Response[any]{}, err
	}
	ctx := RuleContext{Snapshot: o.Value}.withDefaultNow().withDefaultMaps()
	engine := evaluatorEngineName(evaluator)
	start := time.Now()
	value, evalErr := evaluator.Evaluate(ctx, expr)
	duration := time.Since(start)
	evalErr = wrapEvaluationError("", expr, ctx.scopeLabel(), evalErr)
	o.evaluatorLogger().LogEvaluation(EvaluatorLogEvent{
		Engine:   engine,
		Expr:     expr,
		Scope:    ctx.scopeLabel(),
		Duration: duration,
		Err:      evalErr,
	})
	if evalErr != nil {
		return Response[any]{}, evalErr
	}
	return Response[any]{Value: value}, nil
}

// EvaluateWith executes expr using ctx, falling back to the wrapped value when
// ctx.Snapshot is nil.
func (o *Options[T]) EvaluateWith(ctx RuleContext, expr string) (Response[any], error) {
	if expr == "" {
		return Response[any]{}, fmt.Errorf("expression must not be empty")
	}
	evaluator, err := o.resolveEvaluator()
	if err != nil {
		return Response[any]{}, err
	}
	if ctx.Snapshot == nil {
		ctx.Snapshot = o.Value
	}
	ctx = ctx.withDefaultNow().withDefaultMaps()
	engine := evaluatorEngineName(evaluator)
	start := time.Now()
	value, evalErr := evaluator.Evaluate(ctx, expr)
	duration := time.Since(start)
	evalErr = wrapEvaluationError("", expr, ctx.scopeLabel(), evalErr)
	o.evaluatorLogger().LogEvaluation(EvaluatorLogEvent{
		Engine:   engine,
		Expr:     expr,
		Scope:    ctx.scopeLabel(),
		Duration: duration,
		Err:      evalErr,
	})
	if evalErr != nil {
		return Response[any]{}, evalErr
	}
	return Response[any]{Value: value}, nil
}

func (o *Options[T]) resolveEvaluator() (Evaluator, error) {
	evaluator := o.evaluator()
	if evaluator != nil {
		return evaluator, nil
	}
	var exprOpts []ExprEvaluatorOption
	if cache := o.programCache(); cache != nil {
		exprOpts = append(exprOpts, ExprWithProgramCache(cache))
	}
	if registry := o.functionRegistry(); registry != nil {
		exprOpts = append(exprOpts, ExprWithFunctionRegistry(registry))
	}
	defaultEvaluator := NewExprEvaluator(exprOpts...)
	if defaultEvaluator == nil {
		return nil, ErrNoEvaluator
	}
	o.withEvaluator(defaultEvaluator)
	return defaultEvaluator, nil
}

func evaluatorEngineName(e Evaluator) string {
	if e == nil {
		return "unknown"
	}
	switch fmt.Sprintf("%T", e) {
	case "*opts.exprEvaluator":
		return "expr"
	case "*opts.celEvaluator":
		return "cel"
	case "*opts.jsEvaluator":
		return "js"
	default:
		return "custom"
	}
}
