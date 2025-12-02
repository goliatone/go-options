package opts

import (
	"context"
	"testing"

	"github.com/goliatone/go-options/pkg/activity"
)

func TestWithActivityHooksClonesAndFiltersNil(t *testing.T) {
	hook := activity.HookFunc(func(context.Context, activity.Event) error { return nil })

	opts := New(map[string]int{"a": 1}, WithActivityHooks(activity.Hooks{nil, hook}))
	hooks := opts.ActivityHooks()
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}

	// Mutate returned slice and ensure original configuration is unaffected.
	hooks[0] = nil
	again := opts.ActivityHooks()
	if len(again) != 1 || again[0] == nil {
		t.Fatalf("expected cloned hooks unaffected by mutation, got %+v", again)
	}

	value, err := opts.Get("a")
	if err != nil || value != 1 {
		t.Fatalf("expected Get unaffected, got value=%v err=%v", value, err)
	}
}

func TestActivityHooksDefaultNil(t *testing.T) {
	opts := New(map[string]int{"a": 1})
	if hooks := opts.ActivityHooks(); hooks != nil {
		t.Fatalf("expected nil hooks by default, got %+v", hooks)
	}
}

func TestActivityHooksSurviveStackMerge(t *testing.T) {
	hook := activity.HookFunc(func(context.Context, activity.Event) error { return nil })
	layer := NewLayer(NewScope("tenant", 10), map[string]int{"a": 2})
	stack, err := NewStack(layer)
	if err != nil {
		t.Fatalf("stack: %v", err)
	}

	opts, err := stack.Merge(WithActivityHooks(activity.Hooks{hook}))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}

	hooks := opts.ActivityHooks()
	if len(hooks) != 1 {
		t.Fatalf("expected hook to persist through merge, got %d", len(hooks))
	}
}
