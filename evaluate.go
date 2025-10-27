package opts

import (
	"errors"
	"fmt"
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
	ctx := RuleContext{Snapshot: o.Value}
	value, evalErr := evaluator.Evaluate(ctx, expr)
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
	value, evalErr := evaluator.Evaluate(ctx, expr)
	if evalErr != nil {
		return Response[any]{}, evalErr
	}
	return Response[any]{Value: value}, nil
}

func (o *Options[T]) resolveEvaluator() (Evaluator, error) {
	evaluator := o.evaluator()
	if evaluator == nil {
		return nil, ErrNoEvaluator
	}
	return evaluator, nil
}
