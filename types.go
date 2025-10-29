package opts

import "time"

// Options holds a typed options value and evaluator configuration.
type Options[T any] struct {
	Value T

	cfg optionsConfig
}

// Response stores a typed result produced by an evaluator.
type Response[T any] struct {
	Value T
}

// RuleContext carries inputs needed when evaluating an expression.
type RuleContext struct {
	Snapshot any
	Now      *time.Time
	Args     map[string]any
	Metadata map[string]any
	Scope    string
}

func (ctx RuleContext) withDefaultNow() RuleContext {
	if ctx.Now != nil {
		return ctx
	}
	now := time.Now()
	ctx.Now = &now
	return ctx
}

func (ctx RuleContext) timestamp() time.Time {
	ctx = ctx.withDefaultNow()
	return *ctx.Now
}

func (ctx RuleContext) withDefaultMaps() RuleContext {
	if ctx.Args == nil {
		ctx.Args = map[string]any{}
	}
	if ctx.Metadata == nil {
		ctx.Metadata = map[string]any{}
	}
	return ctx
}

func (ctx RuleContext) scopeLabel() string {
	if ctx.Scope == "" {
		return "unknown"
	}
	return ctx.Scope
}

// Evaluator executes expressions against a rule context.
type Evaluator interface {
	Evaluate(ctx RuleContext, expr string) (any, error)
	Compile(expr string, opts ...CompileOption) (CompiledRule, error)
}

// CompiledRule represents a reusable expression program.
type CompiledRule interface {
	Evaluate(ctx RuleContext) (any, error)
}

// CompileOption configures evaluator compile behaviour.
type CompileOption interface {
	applyCompileOption(*compileConfig)
}

type compileConfig struct{}

type compileOptionFunc func(*compileConfig)

func (f compileOptionFunc) applyCompileOption(cfg *compileConfig) {
	if f != nil {
		f(cfg)
	}
}

type Option func(*optionsConfig)

type optionsConfig struct {
	evaluator    Evaluator
	programCache ProgramCache
	functions    *FunctionRegistry
	logger       EvaluatorLogger
}

func applyOptions(opts []Option) optionsConfig {
	cfg := optionsConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

func (o *Options[T]) evaluator() Evaluator {
	return o.cfg.evaluator
}

func (o *Options[T]) withEvaluator(e Evaluator) {
	o.cfg.evaluator = e
}

func (o *Options[T]) evaluatorOrDefault(defaultEvaluator Evaluator) Evaluator {
	if o.cfg.evaluator != nil {
		return o.cfg.evaluator
	}
	return defaultEvaluator
}

func (o *Options[T]) programCache() ProgramCache {
	return o.cfg.programCache
}

func (o *Options[T]) functionRegistry() *FunctionRegistry {
	return o.cfg.functions
}

func (o *Options[T]) evaluatorLogger() EvaluatorLogger {
	if o.cfg.logger != nil {
		return o.cfg.logger
	}
	return noopEvaluatorLogger{}
}
