package activity

import (
	"strings"
	"time"
)

// ScopeContext captures scope metadata associated with an options snapshot.
type ScopeContext struct {
	Name       string
	Label      string
	Priority   int
	Metadata   map[string]any
	SnapshotID string
}

// OptionsEventInput describes the common fields for options lifecycle events.
type OptionsEventInput struct {
	ActorID        string
	UserID         string
	TenantID       string
	ObjectID       string
	Channel        string
	DefinitionCode string
	Recipients     []string
	Metadata       map[string]any
	Path           string
	OldValue       any
	NewValue       any
	Scope          ScopeContext
	OccurredAt     time.Time
}

// BuildOptionsCreatedEvent constructs a normalized activity event for options creation.
func BuildOptionsCreatedEvent(input OptionsEventInput) Event {
	return buildOptionsEvent("options.created", "options", input)
}

// BuildOptionsUpdatedEvent constructs a normalized activity event for options updates.
func BuildOptionsUpdatedEvent(input OptionsEventInput) Event {
	return buildOptionsEvent("options.updated", "options", input)
}

// BuildOptionsDeletedEvent constructs a normalized activity event for options deletion.
func BuildOptionsDeletedEvent(input OptionsEventInput) Event {
	return buildOptionsEvent("options.deleted", "options", input)
}

// BuildOptionsLayerAppliedEvent constructs an activity event describing a layer application.
func BuildOptionsLayerAppliedEvent(input OptionsEventInput) Event {
	return buildOptionsEvent("options.layer.applied", "options.layer", input)
}

func buildOptionsEvent(verb, objectType string, input OptionsEventInput) Event {
	metadata := cloneMap(input.Metadata)
	if input.Path != "" {
		metadata = ensureMetadata(metadata)
		metadata["path"] = input.Path
	}
	if input.Scope.Name != "" {
		metadata = ensureMetadata(metadata)
		metadata["scope_name"] = input.Scope.Name
		metadata["scope_priority"] = input.Scope.Priority
		if input.Scope.Label != "" {
			metadata["scope_label"] = input.Scope.Label
		}
		if len(input.Scope.Metadata) > 0 {
			metadata["scope_metadata"] = cloneMap(input.Scope.Metadata)
		}
	}
	if input.Scope.SnapshotID != "" {
		metadata = ensureMetadata(metadata)
		metadata["snapshot_id"] = input.Scope.SnapshotID
	}
	if input.OldValue != nil {
		metadata = ensureMetadata(metadata)
		metadata["old_value"] = input.OldValue
	}
	if input.NewValue != nil {
		metadata = ensureMetadata(metadata)
		metadata["new_value"] = input.NewValue
	}

	recipients := input.Recipients
	if len(recipients) > 0 {
		recipients = append([]string{}, input.Recipients...)
	}

	objectID := strings.TrimSpace(input.ObjectID)
	if objectID == "" {
		objectID = strings.TrimSpace(input.Path)
	}
	if objectID == "" {
		objectID = strings.TrimSpace(input.Scope.SnapshotID)
	}
	if objectID == "" {
		objectID = objectType
	}

	return Event{
		Verb:           verb,
		ActorID:        strings.TrimSpace(input.ActorID),
		UserID:         strings.TrimSpace(input.UserID),
		TenantID:       strings.TrimSpace(input.TenantID),
		ObjectType:     objectType,
		ObjectID:       objectID,
		Channel:        strings.TrimSpace(input.Channel),
		DefinitionCode: strings.TrimSpace(input.DefinitionCode),
		Recipients:     recipients,
		Metadata:       metadata,
		OccurredAt:     input.OccurredAt,
	}
}

func ensureMetadata(meta map[string]any) map[string]any {
	if meta == nil {
		return map[string]any{}
	}
	return meta
}
