package opts

import (
	"encoding/json"
	"strings"
	"testing"
)

type traceSnapshot struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
	Limits map[string]int    `json:"limits"`
}

func TestResolveWithTraceReturnsLayerProvenance(t *testing.T) {
	defaults := NewLayer(NewScope("defaults", 10), traceSnapshot{
		Name: "defaults",
		Labels: map[string]string{
			"env": "prod",
		},
		Limits: map[string]int{"daily": 100},
	}, WithSnapshotID[traceSnapshot]("defaults/1"))
	user := NewLayer(NewScope("user", 20), traceSnapshot{
		Labels: map[string]string{
			"env":  "staging",
			"team": "core",
		},
		Limits: map[string]int{"daily": 80},
	}, WithSnapshotID[traceSnapshot]("user/5"))

	stack, err := NewStack(defaults, user)
	if err != nil {
		t.Fatalf("stack: %v", err)
	}
	opts, err := stack.Merge()
	if err != nil {
		t.Fatalf("merge: %v", err)
	}

	value, trace, err := opts.ResolveWithTrace("Labels.env")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if value != "staging" {
		t.Fatalf("expected user override, got %v", value)
	}
	if len(trace.Layers) != 2 {
		t.Fatalf("expected 2 provenance entries, got %d", len(trace.Layers))
	}
	if !trace.Layers[0].Found || trace.Layers[0].Scope.Name != "user" {
		t.Fatalf("expected first layer to be user and found, got %+v", trace.Layers[0])
	}
	if !trace.Layers[1].Found || trace.Layers[1].Value != "prod" {
		t.Fatalf("expected defaults layer to provide fallback value, got %+v", trace.Layers[1])
	}
}

func TestResolveWithTraceWithoutStack(t *testing.T) {
	opts := New(map[string]any{
		"feature": map[string]any{"enabled": true},
	})
	value, trace, err := opts.ResolveWithTrace("feature.enabled")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if value != true {
		t.Fatalf("expected true, got %v", value)
	}
	if len(trace.Layers) != 1 || !trace.Layers[0].Found {
		t.Fatalf("expected single synthetic layer, got %+v", trace.Layers)
	}
}

func TestFlattenWithProvenanceEnumeratesPaths(t *testing.T) {
	defaults := NewLayer(NewScope("defaults", 10), traceSnapshot{
		Limits: map[string]int{"daily": 100, "monthly": 500},
	})
	user := NewLayer(NewScope("user", 20), traceSnapshot{
		Limits: map[string]int{"daily": 50},
	})
	stack, err := NewStack(defaults, user)
	if err != nil {
		t.Fatalf("stack: %v", err)
	}
	opts, err := stack.Merge()
	if err != nil {
		t.Fatalf("merge: %v", err)
	}

	if len(opts.layers) == 0 {
		t.Fatalf("expected layer metadata to be attached")
	}

	_, debugTrace, err := opts.ResolveWithTrace("Limits.daily")
	if err != nil {
		t.Fatalf("debug resolve: %v", err)
	}
	if len(debugTrace.Layers) == 0 {
		t.Fatalf("expected trace layers, got %+v", debugTrace)
	}

	results, err := opts.FlattenWithProvenance()
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected flatten results")
	}
	var daily Provenance
	for _, prov := range results {
		if strings.EqualFold(prov.Path, "Limits.daily") && prov.Found {
			daily = prov
			break
		}
	}
	if daily.Scope.Name != "user" {
		t.Fatalf("daily limit should be attributed to user layer, got %+v (trace=%+v results=%+v)", daily.Scope, debugTrace, results)
	}
}

func TestTraceJSONRoundTrip(t *testing.T) {
	trace := Trace{
		Path: "feature.enabled",
		Layers: []Provenance{{
			Scope: Scope{Name: "user"},
			Path:  "feature.enabled",
			Value: true,
			Found: true,
		}},
	}
	raw, err := trace.ToJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("expected valid json, got %s", raw)
	}
	restore, err := TraceFromJSON(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if restore.Path != trace.Path || len(restore.Layers) != len(trace.Layers) {
		t.Fatalf("round trip mismatch: %+v vs %+v", restore, trace)
	}
}
