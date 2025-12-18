package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	opts "github.com/goliatone/go-options"
	"github.com/goliatone/go-options/pkg/state"
)

type Settings struct {
	Notifications Notifications `json:"notifications"`
}

type Notifications struct {
	Email Email `json:"email"`
	SMS   SMS   `json:"sms"`
}

type Email struct {
	Enabled bool   `json:"enabled"`
	Subject string `json:"subject"`
}

type SMS struct {
	Enabled bool `json:"enabled"`
}

func main() {
	ctx := context.Background()
	domain := "notifications"

	store := state.NewMemoryStore[Settings]()
	resolver := state.Resolver[Settings]{Store: store}

	system := opts.NewScope("system", opts.ScopePrioritySystem, opts.WithScopeLabel("System Defaults"))
	team := opts.NewScope("team", opts.ScopePriorityTeam, opts.WithScopeMetadata(map[string]any{"team_id": "team9"}))
	user := opts.NewScope("user", opts.ScopePriorityUser, opts.WithScopeMetadata(map[string]any{"user_id": "u42"}))

	if _, err := store.Save(ctx, state.Ref{Domain: domain, Scope: system}, Settings{
		Notifications: Notifications{
			Email: Email{Enabled: false, Subject: "System template"},
			SMS:   SMS{Enabled: false},
		},
	}, state.Meta{SnapshotID: "snap-system"}); err != nil {
		log.Fatalf("save system: %v", err)
	}

	if _, err := store.Save(ctx, state.Ref{Domain: domain, Scope: team}, Settings{
		Notifications: Notifications{
			Email: Email{Subject: "Team template"},
			SMS:   SMS{Enabled: true},
		},
	}, state.Meta{SnapshotID: "snap-team"}); err != nil {
		log.Fatalf("save team: %v", err)
	}

	if _, err := store.Save(ctx, state.Ref{Domain: domain, Scope: user}, Settings{
		Notifications: Notifications{
			Email: Email{Enabled: true},
		},
	}, state.Meta{SnapshotID: "snap-user"}); err != nil {
		log.Fatalf("save user: %v", err)
	}

	options, err := resolver.Resolve(ctx, domain, user, team, system)
	if err != nil {
		log.Fatalf("resolve: %v", err)
	}

	pretty, _ := json.MarshalIndent(options.Value, "", "  ")
	fmt.Printf("Resolved value:\n%s\n\n", pretty)

	value, trace, err := options.ResolveWithTrace("notifications.email.enabled")
	if err != nil {
		log.Fatalf("trace: %v", err)
	}
	fmt.Printf("Trace for notifications.email.enabled => %v\n", value)
	for _, layer := range trace.Layers {
		fmt.Printf("- %s priority=%d found=%v snapshot=%s\n",
			layer.Scope.Name, layer.Scope.Priority, layer.Found, layer.SnapshotID)
	}

	doc, err := options.Schema()
	if err != nil {
		log.Fatalf("schema: %v", err)
	}
	fmt.Printf("\nSchema scopes:\n")
	for _, scope := range doc.Scopes {
		fmt.Printf("- %s priority=%d snapshot=%s\n", scope.Name, scope.Priority, scope.SnapshotID)
	}

	key, err := (state.Ref{Domain: domain, Scope: user}).Identifier()
	if err != nil {
		log.Fatalf("identifier: %v", err)
	}
	fmt.Printf("\nStorage key example: %s\n", key)
}

