package opts

import (
	"errors"
	"testing"
)

func TestWrapEvaluationErrorCreatesMetadata(t *testing.T) {
	base := errors.New("boom")
	err := wrapEvaluationError("expr", "flag && missing", "user:1", base)

	var evalErr *EvaluationError
	if !errors.As(err, &evalErr) {
		t.Fatalf("expected EvaluationError, got %T", err)
	}
	if evalErr.Engine != "expr" {
		t.Fatalf("expected engine expr, got %q", evalErr.Engine)
	}
	if evalErr.Expr != "flag && missing" {
		t.Fatalf("expected expression metadata, got %q", evalErr.Expr)
	}
	if evalErr.Scope != "user:1" {
		t.Fatalf("expected scope metadata, got %q", evalErr.Scope)
	}
	if !errors.Is(evalErr.Err, base) {
		t.Fatalf("wrapped error should unwrap to base error")
	}
}

func TestWrapEvaluationErrorAugmentsExisting(t *testing.T) {
	base := errors.New("compile failure")
	existing := &EvaluationError{
		Engine: "expr",
		Err:    base,
	}

	err := wrapEvaluationError("cel", "rule", "group:9", existing)
	if !errors.Is(err, base) {
		t.Fatalf("expected base error to unwrap")
	}
	if existing.Engine != "expr" {
		t.Fatalf("existing engine should not be overwritten, got %q", existing.Engine)
	}
	if existing.Expr != "rule" {
		t.Fatalf("expression should be filled, got %q", existing.Expr)
	}
	if existing.Scope != "group:9" {
		t.Fatalf("scope should be filled, got %q", existing.Scope)
	}
}
