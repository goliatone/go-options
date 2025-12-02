package activity

import (
	"context"
	"testing"
)

func TestBuildOptionsUpdatedEventIncludesScopeMetadata(t *testing.T) {
	meta := map[string]any{"custom": "value"}
	scopeMeta := map[string]any{"tenant": "acme"}
	input := OptionsEventInput{
		ActorID:        " actor ",
		UserID:         " user ",
		TenantID:       " tenant ",
		Path:           "features.newUI",
		Metadata:       meta,
		Scope:          ScopeContext{Name: "tenant", Label: "Tenant", Priority: 50, Metadata: scopeMeta, SnapshotID: "snap-1"},
		OldValue:       false,
		NewValue:       true,
		DefinitionCode: "options:update",
		Recipients:     []string{"user@example.com"},
		Channel:        "options",
	}

	event := BuildOptionsUpdatedEvent(input)

	if event.Verb != "options.updated" {
		t.Fatalf("expected verb options.updated got %s", event.Verb)
	}
	if event.ObjectType != "options" || event.ObjectID != "features.newUI" {
		t.Fatalf("unexpected object fields: %+v", event)
	}
	if event.ActorID != "actor" || event.UserID != "user" || event.TenantID != "tenant" {
		t.Fatalf("unexpected identity fields: %+v", event)
	}
	if event.Metadata["path"] != "features.newUI" {
		t.Fatalf("expected path metadata, got %v", event.Metadata["path"])
	}
	if event.Metadata["scope_name"] != "tenant" || event.Metadata["scope_priority"] != 50 {
		t.Fatalf("expected scope metadata, got %+v", event.Metadata)
	}
	if event.Metadata["scope_label"] != "Tenant" {
		t.Fatalf("expected scope_label, got %v", event.Metadata["scope_label"])
	}
	scopeMetadata, ok := event.Metadata["scope_metadata"].(map[string]any)
	if !ok || scopeMetadata["tenant"] != "acme" {
		t.Fatalf("expected scope_metadata clone, got %v", event.Metadata["scope_metadata"])
	}
	if event.Metadata["snapshot_id"] != "snap-1" {
		t.Fatalf("expected snapshot_id, got %v", event.Metadata["snapshot_id"])
	}
	if event.Metadata["old_value"] != false || event.Metadata["new_value"] != true {
		t.Fatalf("expected old/new values, got %v %v", event.Metadata["old_value"], event.Metadata["new_value"])
	}
	if event.DefinitionCode != "options:update" {
		t.Fatalf("expected definition code, got %s", event.DefinitionCode)
	}
	if len(event.Recipients) != 1 || event.Recipients[0] != "user@example.com" {
		t.Fatalf("expected recipients preserved, got %v", event.Recipients)
	}
	event.Recipients[0] = "changed"
	if input.Recipients[0] != "user@example.com" {
		t.Fatalf("expected input recipients untouched, got %v", input.Recipients)
	}
	if meta["custom"] != "value" || scopeMeta["tenant"] != "acme" {
		t.Fatalf("expected input metadata untouched")
	}
}

func TestBuildOptionsDeletedEventUsesFallbackObjectID(t *testing.T) {
	event := BuildOptionsDeletedEvent(OptionsEventInput{})
	if event.ObjectID != "options" {
		t.Fatalf("expected fallback object ID 'options', got %q", event.ObjectID)
	}
}

func TestBuildOptionsLayerAppliedEventPrefersSnapshotID(t *testing.T) {
	input := OptionsEventInput{
		Scope: ScopeContext{
			Name:       "tenant",
			SnapshotID: "snapshot-42",
		},
	}
	event := BuildOptionsLayerAppliedEvent(input)
	if event.Verb != "options.layer.applied" {
		t.Fatalf("expected verb options.layer.applied got %s", event.Verb)
	}
	if event.ObjectType != "options.layer" || event.ObjectID != "snapshot-42" {
		t.Fatalf("unexpected object fields: %+v", event)
	}
	if event.Metadata["snapshot_id"] != "snapshot-42" || event.Metadata["scope_name"] != "tenant" {
		t.Fatalf("expected scope metadata, got %+v", event.Metadata)
	}
}

func TestBuildOptionsEventsWorkWithHooks(t *testing.T) {
	capture := &CaptureHook{}
	hooks := Hooks{capture}

	event := BuildOptionsCreatedEvent(OptionsEventInput{
		Path:     "features.x",
		Scope:    ScopeContext{Name: "user", Priority: 100},
		ObjectID: "",
	})
	err := hooks.Notify(context.Background(), event)
	if err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(capture.Events) != 1 {
		t.Fatalf("expected capture to record event, got %d", len(capture.Events))
	}
	if capture.Events[0].Verb != "options.created" {
		t.Fatalf("expected verb options.created, got %s", capture.Events[0].Verb)
	}
}
