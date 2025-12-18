package opts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	layering "github.com/goliatone/go-options/layering"
)

var errInvalid = errors.New("invalid value")

func skipJSTestsWhenUnavailable(t *testing.T, factoryName string) {
	t.Helper()
	if factoryName == "js" && !jsEvaluatorAvailable() {
		t.Skip("js evaluator requires -tags js_eval")
	}
}

type testValidatable struct {
	Valid bool
}

func (v testValidatable) Validate() error {
	if !v.Valid {
		return errInvalid
	}
	return nil
}

type stubSchemaGenerator struct {
	doc       SchemaDocument
	err       error
	calls     int
	lastValue any
}

func (s *stubSchemaGenerator) Generate(value any) (SchemaDocument, error) {
	s.calls++
	s.lastValue = value
	if s.err != nil {
		return SchemaDocument{}, s.err
	}
	return s.doc, nil
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

func TestOptionsCloneAndWithValue(t *testing.T) {
	type snapshot struct {
		Enabled bool
		Count   int
	}

	cache := &fakeProgramCache{}
	original := New(snapshot{Enabled: true, Count: 1}, WithProgramCache(cache))

	clone := original.Clone()
	if clone == nil {
		t.Fatalf("expected clone to be non-nil")
	}
	if clone == original {
		t.Fatalf("expected clone to return a new pointer")
	}
	if !reflect.DeepEqual(original.Value, clone.Value) {
		t.Fatalf("clone should copy value; want %+v got %+v", original.Value, clone.Value)
	}

	clone.Value.Enabled = false
	clone.Value.Count = 99
	if !original.Value.Enabled || original.Value.Count != 1 {
		t.Fatalf("mutating clone must not affect original; original now %+v", original.Value)
	}
	if clone.programCache() != original.programCache() {
		t.Fatalf("clone should preserve program cache reference")
	}

	updated := original.WithValue(snapshot{Enabled: false, Count: 5})
	if updated == nil {
		t.Fatalf("expected WithValue to return new wrapper")
	}
	if updated == original {
		t.Fatalf("expected WithValue to return a new pointer")
	}
	if !original.Value.Enabled || original.Value.Count != 1 {
		t.Fatalf("WithValue should not mutate original; got %+v", original.Value)
	}
	if updated.Value.Enabled || updated.Value.Count != 5 {
		t.Fatalf("WithValue should set provided value; got %+v", updated.Value)
	}
	if updated.programCache() != original.programCache() {
		t.Fatalf("WithValue should preserve program cache")
	}

	var nilOpts *Options[snapshot]
	if clone := nilOpts.Clone(); clone != nil {
		t.Fatalf("nil Clone should return nil, got %+v", clone)
	}
	if result := nilOpts.WithValue(snapshot{Enabled: true}); result == nil || !result.Value.Enabled {
		t.Fatalf("expected nil WithValue to construct wrapper, got %+v", result)
	}
}

func TestEvaluateWithWrapsEvaluationError(t *testing.T) {
	opts := New(map[string]any{"flag": true})
	ctx := RuleContext{
		ScopeName: "user:42",
	}
	_, err := opts.EvaluateWith(ctx, "flag &&")
	if err == nil {
		t.Fatalf("expected evaluation error")
	}
	var evalErr *EvaluationError
	if !errors.As(err, &evalErr) {
		t.Fatalf("expected EvaluationError, got %T", err)
	}
	if evalErr.Engine != "expr" {
		t.Fatalf("expected engine expr, got %q", evalErr.Engine)
	}
	if evalErr.Expr != "flag &&" {
		t.Fatalf("unexpected expr metadata: %q", evalErr.Expr)
	}
	if evalErr.Scope != "user:42" {
		t.Fatalf("expected scope metadata, got %q", evalErr.Scope)
	}
	if !errors.Is(err, evalErr.Err) {
		t.Fatalf("expected original error to unwrap")
	}
}

func TestEvaluatorLoggerRecordsEvents(t *testing.T) {
	logger := &recordingLogger{}
	opts := New(map[string]any{"flag": true}, WithEvaluatorLogger(logger))

	if _, err := opts.Evaluate("flag"); err != nil {
		t.Fatalf("unexpected evaluate error: %v", err)
	}
	if len(logger.events) != 1 {
		t.Fatalf("expected 1 logged event, got %d", len(logger.events))
	}
	success := logger.events[0]
	if success.Engine != "expr" {
		t.Fatalf("expected engine expr, got %q", success.Engine)
	}
	if success.Err != nil {
		t.Fatalf("expected nil error for successful evaluation, got %v", success.Err)
	}
	if success.Expr != "flag" {
		t.Fatalf("unexpected expression logged: %q", success.Expr)
	}
	if success.Scope != "unknown" {
		t.Fatalf("expected unknown scope by default, got %q", success.Scope)
	}

	if _, err := opts.EvaluateWith(RuleContext{ScopeName: "group:7"}, "flag &&"); err == nil {
		t.Fatalf("expected evaluation error")
	}
	if len(logger.events) != 2 {
		t.Fatalf("expected 2 logged events, got %d", len(logger.events))
	}
	failure := logger.events[1]
	if failure.Scope != "group:7" {
		t.Fatalf("expected scope propagation, got %q", failure.Scope)
	}
	if failure.Expr != "flag &&" {
		t.Fatalf("unexpected expression metadata: %q", failure.Expr)
	}
	var evalErr *EvaluationError
	if failure.Err == nil || !errors.As(failure.Err, &evalErr) {
		t.Fatalf("expected EvaluationError in log event, got %T", failure.Err)
	}
	if evalErr.Engine != "expr" {
		t.Fatalf("expected engine expr in failure, got %q", evalErr.Engine)
	}
}

func TestEvaluateExposesScopeBinding(t *testing.T) {
	opts := New(
		map[string]any{"flag": true},
		WithScope(NewScope("user", ScopePriorityUser, WithScopeMetadata(map[string]any{"tier": "gold"}))),
	)
	resp, err := opts.Evaluate(`scope.name == "user" && scope.metadata.tier == "gold"`)
	if err != nil {
		t.Fatalf("unexpected evaluate error: %v", err)
	}
	value, _ := resp.Value.(bool)
	if !value {
		t.Fatalf("expected expression to see scope bindings, got %v", resp.Value)
	}
}

func TestCELEvaluatorReceivesScopeBinding(t *testing.T) {
	opts := New(
		map[string]any{"flag": true},
		WithEvaluator(NewCELEvaluator()),
		WithScope(NewScope("tenant", ScopePriorityTenant)),
	)
	resp, err := opts.Evaluate(`scope.name == "tenant"`)
	if err != nil {
		t.Fatalf("unexpected evaluate error: %v", err)
	}
	value, _ := resp.Value.(bool)
	if !value {
		t.Fatalf("expected CEL expression to see scope bindings, got %v", resp.Value)
	}
}

func TestOptionsLayerWith(t *testing.T) {
	type snapshot struct {
		Enabled bool
		Tag     string
		Limit   int
	}

	defaults := snapshot{Tag: "defaults", Limit: 10}
	group := snapshot{Tag: "group"}
	user := snapshot{Enabled: true, Limit: 5}

	eval := &capturingEvaluator{}
	original := New(defaults, WithEvaluator(eval))

	layered := original.LayerWith(user, group)
	if layered == nil {
		t.Fatalf("expected layered options to be non-nil")
	}
	if layered == original {
		t.Fatalf("LayerWith must return a new wrapper")
	}

	want := layering.MergeLayers(user, group, defaults)
	if !reflect.DeepEqual(want, layered.Value) {
		t.Fatalf("unexpected layered value\nwant: %#v\n got: %#v", want, layered.Value)
	}

	if !reflect.DeepEqual(defaults, original.Value) {
		t.Fatalf("LayerWith should not mutate original; got %+v", original.Value)
	}

	if layered.evaluator() != original.evaluator() {
		t.Fatalf("LayerWith should preserve evaluator configuration")
	}

	same := original.LayerWith()
	if same == original {
		t.Fatalf("LayerWith without layers should still return a new wrapper")
	}
	if !reflect.DeepEqual(original.Value, same.Value) {
		t.Fatalf("expected no-op layering when no additional layers provided")
	}

	var nilOpts *Options[snapshot]
	if out := nilOpts.LayerWith(user); out == nil || !reflect.DeepEqual(user, out.Value) {
		t.Fatalf("nil LayerWith should hydrate from layers, got %+v", out)
	}
}

func TestSchemaIncludesScopeDescriptorsWhenEnabled(t *testing.T) {
	type snapshot struct {
		Enabled bool `json:"enabled"`
	}
	defaults := NewLayer(NewScope("defaults", ScopePrioritySystem), snapshot{Enabled: false})
	user := NewLayer(NewScope("user", ScopePriorityUser, WithScopeMetadata(map[string]any{"email": "user@example.com"})),
		snapshot{Enabled: true}, WithSnapshotID[snapshot]("user/123"))

	stack, err := NewStack(defaults, user)
	if err != nil {
		t.Fatalf("stack creation failed: %v", err)
	}
	opts, err := stack.Merge(WithScopeSchema(true))
	if err != nil {
		t.Fatalf("stack merge failed: %v", err)
	}
	doc, err := opts.Schema()
	if err != nil {
		t.Fatalf("schema generation failed: %v", err)
	}
	if len(doc.Scopes) != 2 {
		t.Fatalf("expected 2 scope descriptors, got %d", len(doc.Scopes))
	}
	if doc.Scopes[0].Name != "user" || doc.Scopes[0].SnapshotID != "user/123" {
		t.Fatalf("unexpected strongest scope descriptor: %+v", doc.Scopes[0])
	}
	if doc.Scopes[1].Name != "defaults" {
		t.Fatalf("expected defaults scope descriptor, got %+v", doc.Scopes[1])
	}

	optsNoScopes, err := stack.Merge()
	if err != nil {
		t.Fatalf("merge without scopes failed: %v", err)
	}
	doc, err = optsNoScopes.Schema()
	if err != nil {
		t.Fatalf("schema generation failed: %v", err)
	}
	if len(doc.Scopes) != 0 {
		t.Fatalf("scope descriptors should be omitted by default, got %+v", doc.Scopes)
	}
}

func TestSystemTenantOrgTeamUserHelper(t *testing.T) {
	type snapshot struct {
		Timeout int
		Tag     string
	}
	opts, err := SystemTenantOrgTeamUser(
		snapshot{Timeout: 10, Tag: "system"},
		snapshot{Timeout: 20, Tag: "tenant"},
		snapshot{Timeout: 30, Tag: "org"},
		snapshot{Timeout: 40, Tag: "team"},
		snapshot{Timeout: 50, Tag: "user"},
	)
	if err != nil {
		t.Fatalf("helper failed: %v", err)
	}
	if opts.Value.Timeout != 50 || opts.Value.Tag != "user" {
		t.Fatalf("strongest scope should win, got %+v", opts.Value)
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
			skipJSTestsWhenUnavailable(t, factory.name)
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
			skipJSTestsWhenUnavailable(t, factory.name)
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
			skipJSTestsWhenUnavailable(t, factory.name)
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
			skipJSTestsWhenUnavailable(t, factory.name)
			for _, tc := range fx.Cases {
				tc := tc
				t.Run(tc.Name, func(t *testing.T) {
					cache := &fakeProgramCache{}
					evaluator := factory.new(cache, nil)
					snapshot := mergeMaps(fx.Defaults, tc.Input)
					opts := New(snapshot,
						WithEvaluator(evaluator),
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
			skipJSTestsWhenUnavailable(t, factory.name)
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
	doc, err := opts.Schema()
	if err != nil {
		t.Fatalf("Schema returned error: %v", err)
	}
	if doc.Format != SchemaFormatDescriptors {
		t.Fatalf("expected SchemaFormatDescriptors, got %q", doc.Format)
	}

	fields, ok := doc.Document.([]FieldDescriptor)
	if !ok {
		t.Fatalf("expected document to be []FieldDescriptor, got %T", doc.Document)
	}

	got := make(map[string]string, len(fields))
	for _, field := range fields {
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

func TestSchemaUsesConfiguredGenerator(t *testing.T) {
	snapshot := map[string]any{"enabled": true}
	stub := &stubSchemaGenerator{
		doc: SchemaDocument{
			Format:   SchemaFormat("custom"),
			Document: map[string]string{"schema": "custom"},
		},
	}

	opts := New(snapshot, WithSchemaGenerator(stub))

	doc, err := opts.Schema()
	if err != nil {
		t.Fatalf("Schema returned error: %v", err)
	}
	if stub.calls != 1 {
		t.Fatalf("expected generator to be called once, got %d", stub.calls)
	}
	if !reflect.DeepEqual(stub.lastValue, snapshot) {
		t.Fatalf("generator received unexpected value: %+v", stub.lastValue)
	}
	if doc.Format != stub.doc.Format {
		t.Fatalf("expected format %q, got %q", stub.doc.Format, doc.Format)
	}
	if !reflect.DeepEqual(doc.Document, stub.doc.Document) {
		t.Fatalf("expected document %+v, got %+v", stub.doc.Document, doc.Document)
	}
}

func TestSchemaGeneratorErrorIsPropagated(t *testing.T) {
	expectErr := errors.New("boom")
	opts := New(map[string]any{"key": "value"}, WithSchemaGenerator(&stubSchemaGenerator{err: expectErr}))

	if _, err := opts.Schema(); !errors.Is(err, expectErr) {
		t.Fatalf("expected error %v, got %v", expectErr, err)
	}
}

func TestSchemaHandlesNilOptions(t *testing.T) {
	var opts *Options[map[string]any]
	doc, err := opts.Schema()
	if err != nil {
		t.Fatalf("Schema returned error: %v", err)
	}
	if doc.Format != SchemaFormatDescriptors {
		t.Fatalf("expected descriptors format for nil options, got %q", doc.Format)
	}
	fields, ok := doc.Document.([]FieldDescriptor)
	if !ok {
		t.Fatalf("expected []FieldDescriptor, got %T", doc.Document)
	}
	if len(fields) != 0 {
		t.Fatalf("expected empty descriptor list for nil options, got %d", len(fields))
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
			skipJSTestsWhenUnavailable(t, factory.name)
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
	path := filepath.Join("testdata", name)
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

type recordingLogger struct {
	events []EvaluatorLogEvent
}

func (r *recordingLogger) LogEvaluation(event EvaluatorLogEvent) {
	r.events = append(r.events, event)
}
