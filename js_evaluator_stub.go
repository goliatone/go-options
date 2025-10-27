//go:build !js_eval

package opts

// NewJSEvaluator is unavailable without the js_eval build tag.
func NewJSEvaluator(opts ...JSEvaluatorOption) Evaluator {
	_ = applyJSEvaluatorOptions(opts)
	return nil
}

func jsEvaluatorAvailable() bool {
	return false
}
