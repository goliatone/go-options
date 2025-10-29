package opts

import (
	"errors"
	"fmt"
	"strings"
)

// EvaluationError captures evaluator metadata alongside the originating error.
type EvaluationError struct {
	Engine string
	Expr   string
	Scope  string
	Err    error
}

func (e *EvaluationError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("opts: %s evaluator %s scope=%s: %v", e.Engine, describeExpression(e.Expr), e.Scope, e.Err)
}

func (e *EvaluationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func describeExpression(expr string) string {
	if expr == "" {
		return "expr=<empty>"
	}
	return fmt.Sprintf("expr=%q", expr)
}

func wrapEvaluatorError(engine string, err error) error {
	if err == nil {
		return nil
	}

	var evalErr *EvaluationError
	if errors.As(err, &evalErr) {
		return err
	}

	if strings.HasPrefix(err.Error(), "opts:") {
		return err
	}
	return fmt.Errorf("opts: %s evaluator: %w", engine, err)
}

func wrapEvaluationError(engine, expr, scope string, err error) error {
	if err == nil {
		return nil
	}

	var evalErr *EvaluationError
	if errors.As(err, &evalErr) {
		if evalErr.Engine == "" {
			evalErr.Engine = engine
		}
		if evalErr.Expr == "" {
			evalErr.Expr = expr
		}
		if evalErr.Scope == "" {
			evalErr.Scope = scope
		}
		return evalErr
	}

	return &EvaluationError{
		Engine: engine,
		Expr:   expr,
		Scope:  scope,
		Err:    err,
	}
}
