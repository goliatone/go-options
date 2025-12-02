package main

import (
	"context"
	"fmt"

	opts "github.com/goliatone/go-options"
	"github.com/goliatone/go-options/pkg/activity"
	"github.com/goliatone/go-options/pkg/activity/usersink"
	usertypes "github.com/goliatone/go-users/pkg/types"
)

type recordingSink struct {
	records []usertypes.ActivityRecord
}

func (s *recordingSink) Log(_ context.Context, record usertypes.ActivityRecord) error {
	s.records = append(s.records, record)
	return nil
}

func main() {
	capture := &activity.CaptureHook{}
	sink := &recordingSink{}
	hooks := activity.Hooks{
		capture,
		usersink.Hook{Sink: sink},
	}

	optsWrapper := opts.New(
		map[string]any{"features": map[string]any{"newUI": false}},
		opts.WithActivityHooks(hooks),
	)

	event := activity.BuildOptionsUpdatedEvent(activity.OptionsEventInput{
		ActorID:  "user-1",
		UserID:   "user-1",
		TenantID: "tenant-1",
		Path:     "features.newUI",
		OldValue: false,
		NewValue: true,
		Scope: activity.ScopeContext{
			Name:       "tenant",
			Priority:   50,
			SnapshotID: "snap-1",
		},
	})

	_ = optsWrapper.ActivityHooks().Notify(context.Background(), event)

	fmt.Printf("capture events: %d\n", len(capture.Events))
	if len(sink.records) > 0 {
		fmt.Printf("sink records: %d verb=%s object=%s\n", len(sink.records), sink.records[0].Verb, sink.records[0].ObjectID)
	}
}
