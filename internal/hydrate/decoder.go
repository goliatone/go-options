package hydrate

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Context carries identifiers tied to a CMS payload.
type Context struct {
	Slug  string
	Scope string
}

// PreHook lets callers mutate or normalise the payload before decoding.
type PreHook func(Context, map[string]any) (map[string]any, error)

// PostHook lets callers adjust or validate the hydrated struct after decoding.
type PostHook[T any] func(Context, *T) error

// CustomDecoder replaces the default JSON decoding when provided.
type CustomDecoder[T any] func(Context, map[string]any) (T, error)

// DecoderOption configures a Decoder instance.
type DecoderOption[T any] func(*Decoder[T])

// Decoder converts CMS payloads into strongly typed structs.
type Decoder[T any] struct {
	preHooks     []PreHook
	postHooks    []PostHook[T]
	configureDec []func(*json.Decoder)
	custom       CustomDecoder[T]
}

// WithPreHook applies hook prior to decoding.
func WithPreHook[T any](hook PreHook) DecoderOption[T] {
	return func(d *Decoder[T]) {
		d.preHooks = append(d.preHooks, hook)
	}
}

// WithPostHook applies hook after decoding completes.
func WithPostHook[T any](hook PostHook[T]) DecoderOption[T] {
	return func(d *Decoder[T]) {
		d.postHooks = append(d.postHooks, hook)
	}
}

// WithUseNumber enables json.Decoder.UseNumber during decoding.
func WithUseNumber[T any]() DecoderOption[T] {
	return func(d *Decoder[T]) {
		d.configureDec = append(d.configureDec, func(dec *json.Decoder) {
			dec.UseNumber()
		})
	}
}

// WithDisallowUnknownFields invokes json.Decoder.DisallowUnknownFields.
func WithDisallowUnknownFields[T any]() DecoderOption[T] {
	return func(d *Decoder[T]) {
		d.configureDec = append(d.configureDec, func(dec *json.Decoder) {
			dec.DisallowUnknownFields()
		})
	}
}

// WithDecoderConfig allows callers to configure the json.Decoder directly.
func WithDecoderConfig[T any](configure func(*json.Decoder)) DecoderOption[T] {
	return func(d *Decoder[T]) {
		if configure != nil {
			d.configureDec = append(d.configureDec, configure)
		}
	}
}

// WithCustomDecoder replaces the default JSON decoding path.
func WithCustomDecoder[T any](decoder CustomDecoder[T]) DecoderOption[T] {
	return func(d *Decoder[T]) {
		d.custom = decoder
	}
}

func NewDecoder[T any](opts ...DecoderOption[T]) *Decoder[T] {
	d := &Decoder[T]{}
	for _, opt := range opts {
		if opt != nil {
			opt(d)
		}
	}
	return d
}

// Decode converts payload into the target struct T applying configured hooks.
func (d *Decoder[T]) Decode(ctx Context, payload map[string]any) (T, error) {
	var zero T

	if payload == nil {
		return zero, fmt.Errorf("hydrate: payload is nil for slug %q", ctx.Slug)
	}

	current, err := clonePayload(payload)
	if err != nil {
		return zero, fmt.Errorf("hydrate: clone payload for slug %q: %w", ctx.Slug, err)
	}

	for _, hook := range d.preHooks {
		if hook == nil {
			continue
		}
		next, err := hook(ctx, current)
		if err != nil {
			return zero, fmt.Errorf("hydrate: pre-hook for slug %q failed: %w", ctx.Slug, err)
		}
		if next != nil {
			current = next
		}
	}

	var result T
	if d.custom != nil {
		result, err = d.custom(ctx, current)
		if err != nil {
			return zero, fmt.Errorf("hydrate: custom decoder for slug %q failed: %w", ctx.Slug, err)
		}
	} else {
		buffer, err := json.Marshal(current)
		if err != nil {
			return zero, fmt.Errorf("hydrate: marshal payload for slug %q: %w", ctx.Slug, err)
		}
		decoder := json.NewDecoder(bytes.NewReader(buffer))
		for _, configure := range d.configureDec {
			if configure != nil {
				configure(decoder)
			}
		}
		if err := decoder.Decode(&result); err != nil {
			return zero, fmt.Errorf("hydrate: decode slug %q: %w", ctx.Slug, err)
		}
	}

	for _, hook := range d.postHooks {
		if hook == nil {
			continue
		}
		if err := hook(ctx, &result); err != nil {
			return zero, fmt.Errorf("hydrate: post-hook for slug %q failed: %w", ctx.Slug, err)
		}
	}

	return result, nil
}

func clonePayload(payload map[string]any) (map[string]any, error) {
	buffer, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(buffer, &out); err != nil {
		return nil, err
	}
	return out, nil
}
