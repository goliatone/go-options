package opts

import "reflect"

// New constructs an Options wrapper around the provided value.
func New[T any](value T, opts ...Option) *Options[T] {
	cfg := applyOptions(opts)
	return &Options[T]{
		Value: value,
		cfg:   cfg,
	}
}

// Load constructs an Options wrapper and runs validation when supported by the
// underlying type.
func Load[T any](value T, opts ...Option) (*Options[T], error) {
	wrapper := New(value, opts...)
	if err := validateValue(wrapper.Value); err != nil {
		return nil, err
	}
	return wrapper, nil
}

// ApplyDefaults returns value if it is already populated, otherwise it falls
// back to defaults.
func ApplyDefaults[T any](value T, defaults T) T {
	if isZero(value) {
		return defaults
	}
	return value
}

// WithEvaluator configures an evaluator on the Options wrapper.
func WithEvaluator(e Evaluator) Option {
	return func(cfg *optionsConfig) {
		cfg.evaluator = e
	}
}

// Validate invokes the Validate method on the wrapped value when present.
func (o *Options[T]) Validate() error {
	return validateValue(o.Value)
}

func validateValue[T any](value T) error {
	if v, ok := any(value).(interface{ Validate() error }); ok {
		return v.Validate()
	}
	if rv := reflect.ValueOf(value); rv.Kind() != reflect.Pointer && rv.CanAddr() {
		if v, ok := rv.Addr().Interface().(interface{ Validate() error }); ok {
			return v.Validate()
		}
	}
	return nil
}

func isZero[T any](value T) bool {
	return reflect.ValueOf(value).IsZero()
}
