package opts

// ProgramCache stores compiled expression programs keyed by expression strings.
type ProgramCache interface {
	Get(key string) (any, bool)
	Set(key string, value any)
}

// WithProgramCache registers a program cache on the Options wrapper.
func WithProgramCache(cache ProgramCache) Option {
	return func(cfg *optionsConfig) {
		cfg.programCache = cache
	}
}
