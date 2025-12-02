package activity

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNormalizeEventTrimsClonesAndDefaults(t *testing.T) {
	meta := map[string]any{"k": "v"}
	recipients := []string{" a ", "b "}
	evt := Event{
		Verb:           " create ",
		ActorID:        " actor ",
		UserID:         " user ",
		TenantID:       " tenant ",
		ObjectType:     " option ",
		ObjectID:       " 42 ",
		Channel:        " options ",
		DefinitionCode: " def ",
		Recipients:     recipients,
		Metadata:       meta,
	}

	got := NormalizeEvent(evt)

	if got.Verb != "create" || got.ObjectType != "option" || got.ObjectID != "42" {
		t.Fatalf("unexpected normalized fields: %+v", got)
	}
	if got.ActorID != "actor" || got.UserID != "user" || got.TenantID != "tenant" || got.Channel != "options" || got.DefinitionCode != "def" {
		t.Fatalf("unexpected trimming: %+v", got)
	}
	if got.OccurredAt.IsZero() {
		t.Fatalf("expected OccurredAt to be set")
	}
	if got.Metadata["k"] != "v" {
		t.Fatalf("expected metadata value preserved: %+v", got.Metadata)
	}
	got.Metadata["k"] = "changed"
	if evt.Metadata["k"] != "v" {
		t.Fatalf("expected original metadata untouched: %+v", evt.Metadata)
	}
	got.Recipients[0] = "changed"
	if recipients[0] != " a " {
		t.Fatalf("expected original recipients untouched: %+v", recipients)
	}
}

func TestHooksNotifyShortCircuitsMissingRequired(t *testing.T) {
	hooks := Hooks{&CaptureHook{}}
	err := hooks.Notify(context.Background(), Event{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	capture := hooks[0].(*CaptureHook)
	if len(capture.Events) != 0 {
		t.Fatalf("expected no events captured, got %d", len(capture.Events))
	}
}

func TestHooksNotifyFanOutAndJoinErrors(t *testing.T) {
	capture := &CaptureHook{}
	var ctxSeen bool
	hooks := Hooks{
		HookFunc(func(ctx context.Context, event Event) error {
			if ctx != nil {
				ctxSeen = true
			}
			return nil
		}),
		capture,
		HookFunc(func(_ context.Context, _ Event) error { return errors.New("boom1") }),
		nil,
		HookFunc(func(_ context.Context, _ Event) error { return errors.New("boom2") }),
	}

	err := hooks.Notify(nil, Event{Verb: "update", ObjectType: "option", ObjectID: "1"})
	if err == nil || !errors.Is(err, errors.New("boom1")) || !errors.Is(err, errors.New("boom2")) {
		t.Fatalf("expected joined error, got %v", err)
	}
	if !ctxSeen {
		t.Fatalf("expected context fallback to be non-nil")
	}
	if len(capture.Events) != 1 {
		t.Fatalf("expected event to be captured once, got %d", len(capture.Events))
	}
}

func TestEmitterDisabledAndEnabled(t *testing.T) {
	capture := &CaptureHook{}

	disabled := NewEmitter(Hooks{capture}, Config{Enabled: false})
	if disabled.Enabled() {
		t.Fatalf("expected emitter to be disabled")
	}
	if err := disabled.Emit(context.Background(), Event{Verb: "create", ObjectType: "option", ObjectID: "1"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(capture.Events) != 0 {
		t.Fatalf("expected no events captured when disabled")
	}

	enabled := NewEmitter(Hooks{capture}, Config{Enabled: true, Channel: ""})
	if !enabled.Enabled() {
		t.Fatalf("expected emitter to be enabled")
	}
	if err := enabled.Emit(context.Background(), Event{Verb: "create", ObjectType: "option", ObjectID: "1"}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if len(capture.Events) != 1 {
		t.Fatalf("expected one event captured, got %d", len(capture.Events))
	}
	if capture.Events[0].Channel != "options" {
		t.Fatalf("expected default channel applied, got %q", capture.Events[0].Channel)
	}
}

func TestEmitterPreservesExplicitChannel(t *testing.T) {
	capture := &CaptureHook{}
	emitter := NewEmitter(Hooks{capture}, Config{Enabled: true, Channel: "default"})

	err := emitter.Emit(context.Background(), Event{
		Verb:       "create",
		ObjectType: "option",
		ObjectID:   "1",
		Channel:    "custom",
		OccurredAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if capture.Events[0].Channel != "custom" {
		t.Fatalf("expected explicit channel preserved, got %q", capture.Events[0].Channel)
	}
	if capture.Events[0].OccurredAt != (time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected occurred_at preserved, got %v", capture.Events[0].OccurredAt)
	}
}
