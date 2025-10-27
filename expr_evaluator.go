package opts

import (
	"fmt"

	exprlang "github.com/expr-lang/expr"
)

// exprEvaluator executes rule expressions using github.com/expr-lang/expr.
type exprEvaluator struct{}

// NewExprEvaluator constructs an Evaluator backed by expr-lang/expr.
func NewExprEvaluator() Evaluator {
	return &exprEvaluator{}
}

// Evaluate compiles and runs expression against ctx.Snapshot.
func (e *exprEvaluator) Evaluate(ctx RuleContext, expression string) (any, error) {
	if expression == "" {
		return nil, fmt.Errorf("expression must not be empty")
	}
	env := ctx.Snapshot
	if env == nil {
		env = map[string]any{}
	}
	return exprlang.Eval(expression, env)
}

// Compile returns a compiled rule that evaluates expression per invocation.
func (e *exprEvaluator) Compile(expression string, _ ...CompileOption) (CompiledRule, error) {
	if expression == "" {
		return nil, fmt.Errorf("expression must not be empty")
	}
	return &exprCompiledRule{
		evaluator:  e,
		expression: expression,
	}, nil
}

type exprCompiledRule struct {
	evaluator  *exprEvaluator
	expression string
}

func (r *exprCompiledRule) Evaluate(ctx RuleContext) (any, error) {
	return r.evaluator.Evaluate(ctx, r.expression)
}
