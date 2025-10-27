package opts

import "reflect"

// MergeLayers composes snapshots ordered from strongest to weakest, returning a
// new value that keeps explicit settings from stronger layers while filling any
// missing data from weaker ones.
func MergeLayers[T any](layers ...T) T {
	var zero T
	if len(layers) == 0 {
		return zero
	}

	merged := cloneValue(reflect.ValueOf(layers[len(layers)-1]))
	for i := len(layers) - 2; i >= 0; i-- {
		merged = mergeValue(reflect.ValueOf(layers[i]), merged)
	}

	if !merged.IsValid() {
		return zero
	}
	if merged.Type() != reflect.TypeOf(zero) {
		// The merged value might be addressable when T is not; convert back.
		result := reflect.New(reflect.TypeOf(zero)).Elem()
		result.Set(merged.Convert(reflect.TypeOf(zero)))
		return result.Interface().(T)
	}
	return merged.Interface().(T)
}

func mergeValue(strong, weak reflect.Value) reflect.Value {
	if !strong.IsValid() {
		return cloneValue(weak)
	}

	switch strong.Kind() {
	case reflect.Pointer:
		if strong.IsNil() {
			return cloneValue(weak)
		}
		var weakElem reflect.Value
		if weak.IsValid() && weak.Kind() == reflect.Pointer && !weak.IsNil() {
			weakElem = weak.Elem()
		}
		merged := mergeValue(strong.Elem(), weakElem)
		result := reflect.New(strong.Type().Elem())
		result.Elem().Set(merged)
		return result
	case reflect.Interface:
		if strong.IsNil() {
			return cloneValue(weak)
		}
		var weakElem reflect.Value
		if weak.IsValid() && !weak.IsNil() {
			weakElem = weak.Elem()
		}
		merged := mergeValue(strong.Elem(), weakElem)
		return merged.Convert(strong.Type())
	case reflect.Struct:
		result := reflect.New(strong.Type()).Elem()
		var weakStruct reflect.Value
		if weak.IsValid() && weak.Type() == strong.Type() {
			weakStruct = weak
		}
		for i := 0; i < strong.NumField(); i++ {
			field := result.Field(i)
			if !field.CanSet() {
				continue
			}
			var weakField reflect.Value
			if weakStruct.IsValid() {
				weakField = weakStruct.Field(i)
			}
			merged := mergeValue(strong.Field(i), weakField)
			field.Set(merged)
		}
		return result
	case reflect.Map:
		if strong.IsNil() {
			return cloneValue(weak)
		}
		result := reflect.MakeMapWithSize(strong.Type(), strong.Len())
		if weak.IsValid() && weak.Kind() == reflect.Map && !weak.IsNil() {
			iter := weak.MapRange()
			for iter.Next() {
				result.SetMapIndex(iter.Key(), cloneValue(iter.Value()))
			}
		}
		iter := strong.MapRange()
		for iter.Next() {
			key := iter.Key()
			value := iter.Value()
			existing := result.MapIndex(key)
			if existing.IsValid() {
				result.SetMapIndex(key, mergeValue(value, existing))
				continue
			}
			result.SetMapIndex(key, cloneValue(value))
		}
		return result
	case reflect.Slice:
		if strong.IsNil() {
			return cloneValue(weak)
		}
		result := reflect.MakeSlice(strong.Type(), strong.Len(), strong.Len())
		for i := 0; i < strong.Len(); i++ {
			result.Index(i).Set(cloneValue(strong.Index(i)))
		}
		return result
	case reflect.Array:
		result := reflect.New(strong.Type()).Elem()
		for i := 0; i < strong.Len(); i++ {
			var weakElem reflect.Value
			if weak.IsValid() && weak.Kind() == reflect.Array && weak.Len() > i {
				weakElem = weak.Index(i)
			}
			result.Index(i).Set(mergeValue(strong.Index(i), weakElem))
		}
		return result
	default:
		return cloneValue(strong)
	}
}

func cloneValue(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}

	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		clone := reflect.New(v.Type().Elem())
		clone.Elem().Set(cloneValue(v.Elem()))
		return clone
	case reflect.Interface:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		elem := cloneValue(v.Elem())
		if !elem.IsValid() {
			return reflect.Zero(v.Type())
		}
		return elem.Convert(v.Type())
	case reflect.Struct:
		clone := reflect.New(v.Type()).Elem()
		for i := 0; i < v.NumField(); i++ {
			field := clone.Field(i)
			if !field.CanSet() {
				continue
			}
			field.Set(cloneValue(v.Field(i)))
		}
		return clone
	case reflect.Map:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		clone := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			clone.SetMapIndex(iter.Key(), cloneValue(iter.Value()))
		}
		return clone
	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		clone := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			clone.Index(i).Set(cloneValue(v.Index(i)))
		}
		return clone
	case reflect.Array:
		clone := reflect.New(v.Type()).Elem()
		for i := 0; i < v.Len(); i++ {
			clone.Index(i).Set(cloneValue(v.Index(i)))
		}
		return clone
	default:
		return reflect.ValueOf(v.Interface())
	}
}
