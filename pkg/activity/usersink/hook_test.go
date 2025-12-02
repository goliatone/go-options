package usersink_test

import (
	"context"
	"testing"
	"time"

	"github.com/goliatone/go-options/pkg/activity"
	"github.com/goliatone/go-options/pkg/activity/usersink"
	usertypes "github.com/goliatone/go-users/pkg/types"
	"github.com/google/uuid"
)

type recordingSink struct {
	records []usertypes.ActivityRecord
	err     error
}

func (s *recordingSink) Log(_ context.Context, record usertypes.ActivityRecord) error {
	s.records = append(s.records, record)
	return s.err
}

func TestHookNotifyMapsEvent(t *testing.T) {
	sink := &recordingSink{}
	hook := usersink.Hook{Sink: sink}

	now := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	actorID := uuid.New()
	userID := uuid.New()
	tenantID := uuid.New()
	objectID := uuid.New().String()

	event := activity.Event{
		Verb:           "update",
		ActorID:        actorID.String(),
		UserID:         userID.String(),
		TenantID:       tenantID.String(),
		ObjectType:     "option",
		ObjectID:       objectID,
		Channel:        "options",
		DefinitionCode: "option:update",
		Recipients:     []string{"recipient@example.com"},
		Metadata: map[string]any{
			"path": "feature.flag",
		},
		OccurredAt: now,
	}

	if err := hook.Notify(context.Background(), event); err != nil {
		t.Fatalf("notify: %v", err)
	}

	if len(sink.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(sink.records))
	}
	record := sink.records[0]
	if record.ActorID != actorID {
		t.Fatalf("expected actor %s got %s", actorID, record.ActorID)
	}
	if record.UserID != userID {
		t.Fatalf("expected user %s got %s", userID, record.UserID)
	}
	if record.TenantID != tenantID {
		t.Fatalf("expected tenant %s got %s", tenantID, record.TenantID)
	}
	if record.Verb != "update" || record.ObjectType != "option" || record.ObjectID != objectID {
		t.Fatalf("unexpected record payload: %+v", record)
	}
	if record.Channel != "options" {
		t.Fatalf("expected channel options got %q", record.Channel)
	}
	if record.OccurredAt != now {
		t.Fatalf("expected occurred_at %v got %v", now, record.OccurredAt)
	}
	if record.Data["definition_code"] != "option:update" {
		t.Fatalf("expected definition_code metadata got %v", record.Data["definition_code"])
	}
	if record.Data["path"] != "feature.flag" {
		t.Fatalf("expected metadata passthrough got %v", record.Data["path"])
	}
	recipients, ok := record.Data["recipients"].([]string)
	if !ok || len(recipients) != 1 || recipients[0] != "recipient@example.com" {
		t.Fatalf("expected recipients metadata got %v", record.Data["recipients"])
	}
}

func TestHookNotifySkipsMissingVerb(t *testing.T) {
	sink := &recordingSink{}
	hook := usersink.Hook{Sink: sink}

	_ = hook.Notify(context.Background(), activity.Event{})

	if len(sink.records) != 0 {
		t.Fatalf("expected no records for empty event, got %d", len(sink.records))
	}
}

func TestHookNotifyDefaultsTimestamp(t *testing.T) {
	sink := &recordingSink{}
	hook := usersink.Hook{Sink: sink}

	err := hook.Notify(context.Background(), activity.Event{
		Verb:       "create",
		ObjectType: "option",
		ObjectID:   "1",
	})
	if err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(sink.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(sink.records))
	}
	if sink.records[0].OccurredAt.IsZero() {
		t.Fatalf("expected occurred_at to be defaulted")
	}
}
