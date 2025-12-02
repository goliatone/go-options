package opts

import (
	"time"

	"github.com/goliatone/go-options/pkg/activity"
)

// Options holds a typed options value and evaluator configuration.
type Options[T any] struct {
	Value T

	cfg    optionsConfig
	layers []layerSnapshot
}

// SchemaFormat identifies the representation a schema document encodes.
type SchemaFormat string

const (
	// SchemaFormatDescriptors represents the flattened field descriptors.
	SchemaFormatDescriptors SchemaFormat = "descriptors"
	// SchemaFormatOpenAPI represents OpenAPI-compatible JSON Schema documents.
	SchemaFormatOpenAPI SchemaFormat = "openapi"
)

// SchemaDocument encapsulates a generated schema output alongside its format
// identifier. Implementations must ensure Document is JSON-serialisable.
type SchemaDocument struct {
	Format   SchemaFormat
	Document any
	Scopes   []SchemaScope
}

// SchemaScope describes a single scope entry included in a schema document.
type SchemaScope struct {
	Name       string         `json:"name"`
	Label      string         `json:"label,omitempty"`
	Priority   int            `json:"priority"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	SnapshotID string         `json:"snapshot_id,omitempty"`
}

// SchemaGenerator transforms an options value into a schema document. All
// implementations MUST be safe for concurrent use and handle nil inputs by
// returning an empty schema document.
type SchemaGenerator interface {
	Generate(value any) (SchemaDocument, error)
}

// Response stores a typed result produced by an evaluator.
type Response[T any] struct {
	Value T
}

// RuleContext carries inputs needed when evaluating an expression.
type RuleContext struct {
	Snapshot  any
	Now       *time.Time
	Args      map[string]any
	Metadata  map[string]any
	Scope     Scope
	ScopeName string
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

func (ctx RuleContext) withDefaultScope(scope Scope) RuleContext {
	if ctx.Scope.isZero() && !scope.isZero() {
		ctx.Scope = scope.clone()
	}
	if ctx.ScopeName == "" && ctx.Scope.Name != "" {
		ctx.ScopeName = ctx.Scope.Name
	}
	return ctx
}

func (ctx RuleContext) scopeLabel() string {
	if ctx.Scope.Name != "" {
		return ctx.Scope.Name
	}
	if ctx.ScopeName != "" {
		return ctx.ScopeName
	}
	return "unknown"
}

func (ctx RuleContext) scopeBinding() map[string]any {
	if binding := scopeToBinding(ctx.Scope); binding != nil {
		return binding
	}
	if ctx.ScopeName == "" {
		return nil
	}
	return map[string]any{"name": ctx.ScopeName}
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
	evaluator       Evaluator
	programCache    ProgramCache
	functions       *FunctionRegistry
	logger          EvaluatorLogger
	schemaGenerator SchemaGenerator
	scope           Scope
	scopeSchema     bool
	activityHooks   activity.Hooks
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

// WithSchemaGenerator configures a custom schema generator implementation.
func WithSchemaGenerator(generator SchemaGenerator) Option {
	return func(cfg *optionsConfig) {
		cfg.schemaGenerator = generator
	}
}

// WithScope configures the default scope metadata applied to evaluator contexts.
func WithScope(scope Scope) Option {
	return func(cfg *optionsConfig) {
		cfg.scope = scope.clone()
	}
}

// WithScopeSchema toggles inclusion of scope metadata within generated schemas.
func WithScopeSchema(include bool) Option {
	return func(cfg *optionsConfig) {
		cfg.scopeSchema = include
	}
}

func scopeToBinding(scope Scope) map[string]any {
	if scope.isZero() {
		return nil
	}
	binding := map[string]any{
		"name":     scope.Name,
		"label":    scope.Label,
		"priority": scope.Priority,
	}
	if len(scope.Metadata) > 0 {
		binding["metadata"] = copyMetadata(scope.Metadata)
	}
	return binding
}

func (o *Options[T]) schemaGenerator() SchemaGenerator {
	if o == nil {
		return DefaultSchemaGenerator()
	}
	if o.cfg.schemaGenerator != nil {
		return o.cfg.schemaGenerator
	}
	return DefaultSchemaGenerator()
}
