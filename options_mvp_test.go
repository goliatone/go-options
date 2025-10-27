package opts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

var errInvalid = errors.New("invalid value")

type testValidatable struct {
	Valid bool
}

func (v testValidatable) Validate() error {
	if !v.Valid {
		return errInvalid
	}
	return nil
}

var evaluatorFactories = []struct {
	name string
	new  func(cache ProgramCache, registry *FunctionRegistry) Evaluator
}{
	{
		name: "expr",
		new: func(cache ProgramCache, registry *FunctionRegistry) Evaluator {
			opts := []ExprEvaluatorOption{}
			if cache != nil {
				opts = append(opts, ExprWithProgramCache(cache))
			}
			if registry != nil {
				opts = append(opts, ExprWithFunctionRegistry(registry))
			}
			return NewExprEvaluator(opts...)
		},
	},
	{
		name: "cel",
		new: func(cache ProgramCache, registry *FunctionRegistry) Evaluator {
			opts := []CELEvaluatorOption{}
			if cache != nil {
				opts = append(opts, CELWithProgramCache(cache))
			}
			if registry != nil {
				opts = append(opts, CELWithFunctionRegistry(registry))
			}
			return NewCELEvaluator(opts...)
		},
	},
	{
		name: "js",
		new: func(cache ProgramCache, registry *FunctionRegistry) Evaluator {
			opts := []JSEvaluatorOption{}
			if cache != nil {
				opts = append(opts, JSWithProgramCache(cache))
			}
			if registry != nil {
				opts = append(opts, JSWithFunctionRegistry(registry))
			}
			return NewJSEvaluator(opts...)
		},
	},
}

func TestApplyDefaultsBehaviour(t *testing.T) {
	type config struct {
		Enabled bool
	}

	defaults := config{Enabled: true}
	if got := ApplyDefaults(config{}, defaults); !got.Enabled {
		t.Fatalf("expected ApplyDefaults to return defaults when value is zero")
	}

	type pointerConfig struct {
		Enabled *bool
	}
	falsePtr := func(b bool) *bool { return &b }(false)
	truePtr := func(b bool) *bool { return &b }(true)
	pdefaults := pointerConfig{Enabled: truePtr}
	original := pointerConfig{Enabled: falsePtr}
	if got := ApplyDefaults(original, pdefaults); got.Enabled == nil || *got.Enabled {
		t.Fatalf("expected explicit pointer value to remain unchanged; got %+v", got)
	}
}

func TestLoadRunsValidation(t *testing.T) {
	if _, err := Load(testValidatable{Valid: false}); err == nil {
		t.Fatalf("expected Load to surface validation error")
	} else if !errors.Is(err, errInvalid) {
		t.Fatalf("unexpected validation error: %v", err)
	}

	if _, err := Load(testValidatable{Valid: true}); err != nil {
		t.Fatalf("unexpected error from Load: %v", err)
	}
}

func TestRuleContextDefaultsNow(t *testing.T) {
	capture := &capturingEvaluator{}
	opts := New(map[string]any{}, WithEvaluator(capture))

	if _, err := opts.Evaluate("1 == 1"); err != nil {
		t.Fatalf("unexpected error from Evaluate: %v", err)
	}
	if len(capture.contexts) != 1 {
		t.Fatalf("expected evaluator to receive one context, got %d", len(capture.contexts))
	}
	if capture.contexts[0].Now == nil || capture.contexts[0].Now.IsZero() {
		t.Fatalf("expected Evaluate to default RuleContext.Now")
	}

	capture.reset()

	ctx := RuleContext{
		Snapshot: map[string]any{"flag": true},
	}
	if _, err := opts.EvaluateWith(ctx, "flag"); err != nil {
		t.Fatalf("unexpected error from EvaluateWith: %v", err)
	}
	if len(capture.contexts) != 1 {
		t.Fatalf("expected evaluator to receive one context during EvaluateWith, got %d", len(capture.contexts))
	}
	if capture.contexts[0].Now == nil || capture.contexts[0].Now.IsZero() {
		t.Fatalf("expected EvaluateWith to default RuleContext.Now")
	}
}

func TestUC1FeatureToggleFixture(t *testing.T) {
	type expect struct {
		Value bool   `json:"value"`
		Err   string `json:"err"`
	}
	type testCase struct {
		Name   string         `json:"name"`
		Rule   string         `json:"rule"`
		Input  map[string]any `json:"input"`
		Expect expect         `json:"expect"`
		Notes  string         `json:"notes"`
	}
	type fixture struct {
		Description string         `json:"description"`
		Defaults    map[string]any `json:"defaults"`
		Cases       []testCase     `json:"cases"`
	}

	fx := loadFixture[fixture](t, "uc1_feature_toggle.json")

	for _, factory := range evaluatorFactories {
		factory := factory
		t.Run(factory.name, func(t *testing.T) {
			for _, tc := range fx.Cases {
				tc := tc
				t.Run(tc.Name, func(t *testing.T) {
					snapshot := mergeMaps(fx.Defaults, tc.Input)
					opts := New(snapshot, WithEvaluator(factory.new(nil, nil)))
					resp, err := opts.Evaluate(tc.Rule)

					if tc.Expect.Err != "" {
						if err == nil {
							t.Fatalf("expected error %q but got nil", tc.Expect.Err)
						}
						if err.Error() != tc.Expect.Err {
							t.Fatalf("expected error %q, got %q", tc.Expect.Err, err.Error())
						}
						return
					}

					if err != nil {
						t.Fatalf("unexpected error from Evaluate: %v", err)
					}

					value, ok := resp.Value.(bool)
					if !ok {
						t.Fatalf("expected bool response, got %T", resp.Value)
					}
					if value != tc.Expect.Value {
						t.Fatalf("expected %v, got %v", tc.Expect.Value, value)
					}
				})
			}
		})
	}
}

func TestUC2ChannelSelectionFixture(t *testing.T) {
	type expect struct {
		ActiveChannels   []string `json:"activeChannels"`
		InactiveChannels []string `json:"inactiveChannels"`
	}
	type testCase struct {
		Name   string         `json:"name"`
		Input  map[string]any `json:"input"`
		Expect expect         `json:"expect"`
		Notes  string         `json:"notes"`
	}
	type fixture struct {
		Description string         `json:"description"`
		Defaults    map[string]any `json:"defaults"`
		Cases       []testCase     `json:"cases"`
	}

	fx := loadFixture[fixture](t, "uc2_channel_selection.json")

	for _, factory := range evaluatorFactories {
		factory := factory
		t.Run(factory.name, func(t *testing.T) {
			for _, tc := range fx.Cases {
				tc := tc
				t.Run(tc.Name, func(t *testing.T) {
					snapshot := mergeMaps(fx.Defaults, tc.Input)
					opts := New(snapshot, WithEvaluator(factory.new(nil, nil)))

					names := collectChannelNames(snapshot)
					active := make([]string, 0, len(names))
					inactive := make([]string, 0, len(names))

					for _, name := range names {
						rule := fmt.Sprintf("Channels.%s.Enabled", name)
						resp, err := opts.Evaluate(rule)
						if err != nil {
							t.Fatalf("unexpected error from Evaluate(%q): %v", rule, err)
						}
						enabled, ok := resp.Value.(bool)
						if !ok {
							t.Fatalf("expected bool result for %q, got %T", rule, resp.Value)
						}
						if enabled {
							active = append(active, name)
						} else {
							inactive = append(inactive, name)
						}
					}

					slices.Sort(active)
					slices.Sort(inactive)

					expectedActive := append([]string(nil), tc.Expect.ActiveChannels...)
					expectedInactive := append([]string(nil), tc.Expect.InactiveChannels...)
					slices.Sort(expectedActive)
					slices.Sort(expectedInactive)

					if !slices.Equal(active, expectedActive) {
						t.Fatalf("active channels mismatch, expected %v, got %v", expectedActive, active)
					}
					if !slices.Equal(inactive, expectedInactive) {
						t.Fatalf("inactive channels mismatch, expected %v, got %v", expectedInactive, inactive)
					}
				})
			}
		})
	}
}

func TestUC3TimeRulesFixture(t *testing.T) {
	type expect struct {
		Value bool   `json:"value"`
		Err   string `json:"err"`
	}
	type testCase struct {
		Name    string         `json:"name"`
		Rule    string         `json:"rule"`
		Input   map[string]any `json:"input"`
		Context map[string]any `json:"context"`
		Expect  expect         `json:"expect"`
		Notes   string         `json:"notes"`
	}
	type fixture struct {
		Description string         `json:"description"`
		Defaults    map[string]any `json:"defaults"`
		Cases       []testCase     `json:"cases"`
	}

	fx := loadFixture[fixture](t, "uc3_time_rules.json")

	for _, factory := range evaluatorFactories {
		factory := factory
		t.Run(factory.name, func(t *testing.T) {
			for _, tc := range fx.Cases {
				tc := tc
				t.Run(tc.Name, func(t *testing.T) {
					snapshot := convertTimeEncodings(t, mergeMaps(fx.Defaults, tc.Input)).(map[string]any)
					opts := New(snapshot, WithEvaluator(factory.new(nil, nil)))

					ctx := RuleContext{
						Snapshot: snapshot,
					}
					if tc.Context != nil {
						contextValues := convertTimeEncodings(t, tc.Context).(map[string]any)
						applyTimeContext(&ctx, contextValues)
					}

					resp, err := opts.EvaluateWith(ctx, tc.Rule)

					if tc.Expect.Err != "" {
						if err == nil {
							t.Fatalf("expected error %q but got nil", tc.Expect.Err)
						}
						if err.Error() != tc.Expect.Err {
							t.Fatalf("expected error %q, got %q", tc.Expect.Err, err.Error())
						}
						return
					}

					if err != nil {
						t.Fatalf("unexpected error from EvaluateWith: %v", err)
					}

					value, ok := resp.Value.(bool)
					if !ok {
						t.Fatalf("expected bool response, got %T", resp.Value)
					}
					if value != tc.Expect.Value {
						t.Fatalf("expected %v, got %v", tc.Expect.Value, value)
					}
				})
			}
		})
	}
}

func TestEvaluatorProgramCache(t *testing.T) {
	type cacheExpect struct {
		Hits   int `json:"hits"`
		Misses int `json:"misses"`
	}
	type cacheCase struct {
		Name       string         `json:"name"`
		Rule       string         `json:"rule"`
		Input      map[string]any `json:"input"`
		Iterations int            `json:"iterations"`
		Expect     cacheExpect    `json:"expect"`
	}
	type cacheFixture struct {
		Description string         `json:"description"`
		Defaults    map[string]any `json:"defaults"`
		Cases       []cacheCase    `json:"cases"`
	}

	fx := loadFixture[cacheFixture](t, "cache_programs.json")

	for _, factory := range evaluatorFactories {
		factory := factory
		t.Run(factory.name, func(t *testing.T) {
			for _, tc := range fx.Cases {
				tc := tc
				t.Run(tc.Name, func(t *testing.T) {
					cache := &fakeProgramCache{}
					evaluator := factory.new(cache, nil)
					snapshot := mergeMaps(fx.Defaults, tc.Input)
					opts := New(snapshot,
						WithEvaluator(evaluator),
						WithProgramCache(cache),
					)

					for i := 0; i < tc.Iterations; i++ {
						if _, err := opts.Evaluate(tc.Rule); err != nil {
							t.Fatalf("unexpected error on iteration %d: %v", i, err)
						}
					}

					if cache.hits != tc.Expect.Hits {
						t.Fatalf("cache hits mismatch, expected %d, got %d", tc.Expect.Hits, cache.hits)
					}
					if cache.misses != tc.Expect.Misses {
						t.Fatalf("cache misses mismatch, expected %d, got %d", tc.Expect.Misses, cache.misses)
					}
				})
			}
		})
	}
}

func TestEvaluateWithSnapshotOnlyContext(t *testing.T) {
	for _, factory := range evaluatorFactories {
		factory := factory
		t.Run(factory.name, func(t *testing.T) {
			opts := New(map[string]any{
				"Features": map[string]any{
					"NewUI": map[string]any{
						"Enabled": false,
					},
				},
			}, WithEvaluator(factory.new(nil, nil)))

			override := map[string]any{
				"Features": map[string]any{
					"NewUI": map[string]any{
						"Enabled": true,
					},
				},
			}

			ctx := RuleContext{
				Snapshot: override,
			}

			resp, err := opts.EvaluateWith(ctx, "Features.NewUI.Enabled")
			if err != nil {
				t.Fatalf("unexpected error from EvaluateWith: %v", err)
			}
			value, ok := resp.Value.(bool)
			if !ok {
				t.Fatalf("expected bool response, got %T", resp.Value)
			}
			if !value {
				t.Fatalf("expected EvaluateWith to respect snapshot context override")
			}
		})
	}
}

func TestDynamicPathHelpers(t *testing.T) {
	type readOp struct {
		Name   string `json:"name"`
		Path   string `json:"path"`
		Expect bool   `json:"expect"`
	}
	type writeOp struct {
		Name   string `json:"name"`
		Path   string `json:"path"`
		Value  any    `json:"value"`
		Expect bool   `json:"expect"`
	}
	type fixture struct {
		Snapshot map[string]any `json:"snapshot"`
		Reads    []readOp       `json:"reads"`
		Writes   []writeOp      `json:"writes"`
	}

	fx := loadFixture[fixture](t, "dynamic_paths.json")
	snapshot := cloneMap(fx.Snapshot)
	opts := New(snapshot)

	for _, op := range fx.Reads {
		value, err := opts.Get(op.Path)
		if err != nil {
			t.Fatalf("read %q failed: %v", op.Name, err)
		}
		got, ok := value.(bool)
		if !ok {
			t.Fatalf("read %q expected bool, got %T", op.Name, value)
		}
		if got != op.Expect {
			t.Fatalf("read %q expected %v, got %v", op.Name, op.Expect, got)
		}
	}

	for _, op := range fx.Writes {
		if err := opts.Set(op.Path, op.Value); err != nil {
			t.Fatalf("write %q failed: %v", op.Name, err)
		}
		value, err := opts.Get(op.Path)
		if err != nil {
			t.Fatalf("write %q readback failed: %v", op.Name, err)
		}
		got, ok := value.(bool)
		if !ok {
			t.Fatalf("write %q readback expected bool, got %T", op.Name, value)
		}
		if got != op.Expect {
			t.Fatalf("write %q expected %v, got %v", op.Name, op.Expect, got)
		}
	}
}

func TestSchemaGeneration(t *testing.T) {
	type descriptor struct {
		Path string `json:"path"`
		Type string `json:"type"`
	}
	type fixture struct {
		Snapshot map[string]any `json:"snapshot"`
		Expect   struct {
			Fields []descriptor `json:"fields"`
		} `json:"expect"`
	}

	fx := loadFixture[fixture](t, "schema_fields.json")
	opts := New(cloneMap(fx.Snapshot))
	schema := opts.Schema()

	got := make(map[string]string, len(schema.Fields))
	for _, field := range schema.Fields {
		got[field.Path] = field.Type
	}

	if len(got) != len(fx.Expect.Fields) {
		t.Fatalf("expected %d schema fields, got %d", len(fx.Expect.Fields), len(got))
	}

	for _, field := range fx.Expect.Fields {
		typ, exists := got[field.Path]
		if !exists {
			t.Fatalf("expected schema to contain path %q", field.Path)
		}
		if typ != field.Type {
			t.Fatalf("path %q expected type %q, got %q", field.Path, field.Type, typ)
		}
	}
}

func TestCustomFunctionsAcrossEvaluators(t *testing.T) {
	type expect struct {
		Value bool   `json:"value"`
		Err   string `json:"err"`
	}
	type testCase struct {
		Name     string         `json:"name"`
		Rule     string         `json:"rule"`
		Input    map[string]any `json:"input"`
		Metadata map[string]any `json:"metadata"`
		Args     map[string]any `json:"args"`
		Context  map[string]any `json:"context"`
		Expect   expect         `json:"expect"`
	}
	type fixture struct {
		Defaults map[string]any `json:"defaults"`
		Cases    []testCase     `json:"cases"`
	}

	fx := loadFixture[fixture](t, "custom_functions.json")
	defaults := convertTimeEncodings(t, fx.Defaults).(map[string]any)

	for _, factory := range evaluatorFactories {
		factory := factory
		t.Run(factory.name, func(t *testing.T) {
			registry := NewFunctionRegistry()
			if err := registry.Register("equalsIgnoreCase", func(args ...any) (any, error) {
				if len(args) != 2 {
					return nil, fmt.Errorf("equalsIgnoreCase expects 2 args")
				}
				a, _ := args[0].(string)
				b, _ := args[1].(string)
				return strings.EqualFold(a, b), nil
			}); err != nil {
				t.Fatalf("register equalsIgnoreCase: %v", err)
			}
			if err := registry.Register("withinQuietHours", func(args ...any) (any, error) {
				if len(args) != 3 {
					return nil, fmt.Errorf("withinQuietHours expects 3 args")
				}
				now, ok := args[0].(time.Time)
				if !ok {
					return nil, fmt.Errorf("now must be time.Time")
				}
				start, ok := args[1].(time.Time)
				if !ok {
					return nil, fmt.Errorf("start must be time.Time")
				}
				end, ok := args[2].(time.Time)
				if !ok {
					return nil, fmt.Errorf("end must be time.Time")
				}
				return (now.Equal(start) || now.After(start)) && now.Before(end), nil
			}); err != nil {
				t.Fatalf("register withinQuietHours: %v", err)
			}

			for _, tc := range fx.Cases {
				tc := tc
				t.Run(tc.Name, func(t *testing.T) {
					input := convertTimeEncodings(t, tc.Input)
					var inputMap map[string]any
					if input != nil {
						inputMap, _ = toStringMap(input)
					}
					snapshot := mergeMaps(defaults, inputMap)
					opts := New(snapshot,
						WithFunctionRegistry(registry),
						WithEvaluator(factory.new(nil, registry)),
					)

					ctx := RuleContext{
						Snapshot: snapshot,
					}
					if tc.Metadata != nil {
						if metadata, ok := toStringMap(convertTimeEncodings(t, tc.Metadata)); ok {
							ctx.Metadata = metadata
						}
					}
					if tc.Args != nil {
						if args, ok := toStringMap(convertTimeEncodings(t, tc.Args)); ok {
							ctx.Args = args
						}
					}
					if tc.Context != nil {
						contextValues := convertTimeEncodings(t, tc.Context)
						if mapped, ok := toStringMap(contextValues); ok {
							applyTimeContext(&ctx, mapped)
						}
					}

					resp, err := opts.EvaluateWith(ctx, tc.Rule)

					if tc.Expect.Err != "" {
						if err == nil {
							t.Fatalf("expected error %q but got nil", tc.Expect.Err)
						}
						if err.Error() != tc.Expect.Err {
							t.Fatalf("expected error %q, got %q", tc.Expect.Err, err.Error())
						}
						return
					}

					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					value, ok := resp.Value.(bool)
					if !ok {
						t.Fatalf("expected bool value, got %T", resp.Value)
					}
					if value != tc.Expect.Value {
						t.Fatalf("expected %v, got %v", tc.Expect.Value, value)
					}
				})
			}
		})
	}
}

func loadFixture[T any](t *testing.T, name string) T {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to resolve caller for fixture %q", name)
	}
	path := filepath.Join(filepath.Dir(file), "testdata", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %q: %v", path, err)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("failed to unmarshal fixture %q: %v", path, err)
	}
	return out
}

func collectChannelNames(snapshot map[string]any) []string {
	channels, ok := snapshot["Channels"].(map[string]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(channels))
	for name := range channels {
		names = append(names, name)
	}
	return names
}

func mergeMaps(base, override map[string]any) map[string]any {
	out := cloneMap(base)
	for key, value := range override {
		if existing, ok := out[key]; ok {
			if existingMap, ok := toStringMap(existing); ok {
				if overrideMap, ok := toStringMap(value); ok {
					out[key] = mergeMaps(existingMap, overrideMap)
					continue
				}
			}
		}
		out[key] = cloneValue(value)
	}
	return out
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = cloneValue(value)
	}
	return out
}

func cloneValue(value any) any {
	if m, ok := toStringMap(value); ok {
		return cloneMap(m)
	}
	if slice, ok := value.([]any); ok {
		out := make([]any, len(slice))
		for i, item := range slice {
			out[i] = cloneValue(item)
		}
		return out
	}
	return value
}

func toStringMap(value any) (map[string]any, bool) {
	m, ok := value.(map[string]any)
	return m, ok
}

func convertTimeEncodings(t *testing.T, value any) any {
	t.Helper()
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, val := range v {
			out[key] = convertTimeEncodings(t, val)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, val := range v {
			out[i] = convertTimeEncodings(t, val)
		}
		return out
	case string:
		const prefix = "time:"
		if strings.HasPrefix(v, prefix) {
			ts, err := time.Parse(time.RFC3339, strings.TrimPrefix(v, prefix))
			if err != nil {
				t.Fatalf("invalid time encoding %q: %v", v, err)
			}
			return ts
		}
	}
	return value
}

func applyTimeContext(ctx *RuleContext, values map[string]any) {
	if values == nil {
		return
	}
	if nowValue, ok := values["now"].(time.Time); ok {
		ctx.Now = &nowValue
	}
}

type fakeProgramCache struct {
	store  map[string]any
	hits   int
	misses int
}

func (c *fakeProgramCache) Get(key string) (any, bool) {
	if c.store == nil {
		c.store = make(map[string]any)
	}
	value, ok := c.store[key]
	if ok {
		c.hits++
		return value, true
	}
	c.misses++
	return nil, false
}

func (c *fakeProgramCache) Set(key string, value any) {
	if c.store == nil {
		c.store = make(map[string]any)
	}
	c.store[key] = value
}

type capturingEvaluator struct {
	contexts []RuleContext
}

func (c *capturingEvaluator) Evaluate(ctx RuleContext, _ string) (any, error) {
	c.contexts = append(c.contexts, ctx)
	return true, nil
}

func (c *capturingEvaluator) Compile(string, ...CompileOption) (CompiledRule, error) {
	return nil, fmt.Errorf("capturing evaluator does not support compile")
}

func (c *capturingEvaluator) reset() {
	c.contexts = c.contexts[:0]
}
