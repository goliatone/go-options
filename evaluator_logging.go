package opts

import "time"

// EvaluatorLogEvent describes an evaluation attempt for logging.
type EvaluatorLogEvent struct {
	Engine   string
	Expr     string
	Scope    string
	Duration time.Duration
	Err      error
}

// EvaluatorLogger records evaluator events.
type EvaluatorLogger interface {
	LogEvaluation(EvaluatorLogEvent)
}

// EvaluatorLoggerFunc adapts a function to EvaluatorLogger.
type EvaluatorLoggerFunc func(EvaluatorLogEvent)

// LogEvaluation implements EvaluatorLogger.
func (f EvaluatorLoggerFunc) LogEvaluation(event EvaluatorLogEvent) {
	if f != nil {
		f(event)
	}
}

type noopEvaluatorLogger struct{}

func (noopEvaluatorLogger) LogEvaluation(EvaluatorLogEvent) {}

// WithEvaluatorLogger attaches an evaluator logger to the Options wrapper.
func WithEvaluatorLogger(logger EvaluatorLogger) Option {
	return func(cfg *optionsConfig) {
		if logger == nil {
			cfg.logger = noopEvaluatorLogger{}
			return
		}
		cfg.logger = logger
	}
}
