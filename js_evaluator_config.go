package opts

type jsEvaluatorConfig struct {
	cache    ProgramCache
	registry *FunctionRegistry
}

// JSEvaluatorOption configures the JS evaluator.
type JSEvaluatorOption func(*jsEvaluatorConfig)

// JSWithProgramCache applies a ProgramCache to the JS evaluator.
func JSWithProgramCache(cache ProgramCache) JSEvaluatorOption {
	return func(cfg *jsEvaluatorConfig) {
		cfg.cache = cache
	}
}

// JSWithFunctionRegistry applies a FunctionRegistry to the JS evaluator.
func JSWithFunctionRegistry(registry *FunctionRegistry) JSEvaluatorOption {
	return func(cfg *jsEvaluatorConfig) {
		if registry == nil {
			return
		}
		cfg.registry = registry.Clone()
	}
}

func applyJSEvaluatorOptions(opts []JSEvaluatorOption) jsEvaluatorConfig {
	cfg := jsEvaluatorConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}
