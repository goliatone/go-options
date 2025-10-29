package opts

import layering "github.com/goliatone/opts/layering"

// LayerWith merges layers ordered strongest to weakest with the current
// snapshot as the fallback, returning a new wrapper with the merged value.
func (o *Options[T]) LayerWith(layers ...T) *Options[T] {
	if o == nil {
		if len(layers) == 0 {
			return nil
		}
		merged := layering.MergeLayers(layers...)
		return &Options[T]{Value: merged}
	}

	combined := append([]T(nil), layers...)
	combined = append(combined, o.Value)
	merged := layering.MergeLayers(combined...)
	return o.WithValue(merged)
}
