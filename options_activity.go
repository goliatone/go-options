package opts

import "github.com/goliatone/go-options/pkg/activity"

// WithActivityHooks attaches activity hooks to the Options configuration.
// Hooks are cloned and nil entries dropped to preserve immutability.
func WithActivityHooks(hooks activity.Hooks) Option {
	normalized := cloneActivityHooks(hooks)
	return func(cfg *optionsConfig) {
		cfg.activityHooks = normalized
	}
}

// ActivityHooks returns a cloned slice of activity hooks configured on the
// options wrapper. The returned slice can be safely mutated by the caller.
func (o *Options[T]) ActivityHooks() activity.Hooks {
	if o == nil {
		return nil
	}
	return cloneActivityHooks(o.cfg.activityHooks)
}

func cloneActivityHooks(hooks activity.Hooks) activity.Hooks {
	if len(hooks) == 0 {
		return nil
	}
	normalized := make([]activity.ActivityHook, 0, len(hooks))
	for _, hook := range hooks {
		if hook == nil {
			continue
		}
		normalized = append(normalized, hook)
	}
	if len(normalized) == 0 {
		return nil
	}
	return activity.Hooks(normalized)
}
