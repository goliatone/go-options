package hydrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDecoderFromFixtures(t *testing.T) {
	fx := loadFixture(t, "hydrate_notifications.json")

	for _, tc := range fx.Cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			options := buildOptions(tc)
			decoder := NewDecoder[notificationSettings](options...)

			ctx := Context{
				Slug:  tc.Slug,
				Scope: tc.Scope,
			}

			result, err := decoder.Decode(ctx, tc.Input)

			if tc.ExpectErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tc.ExpectErr)
				}
				if !strings.Contains(err.Error(), tc.ExpectErr) {
					t.Fatalf("expected error containing %q, got %v", tc.ExpectErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected decode error: %v", err)
			}

			if !reflect.DeepEqual(tc.Expect, result) {
				t.Fatalf("decoded snapshot mismatch:\nwant: %#v\n got: %#v", tc.Expect, result)
			}
		})
	}
}

func buildOptions(tc fixtureCase) []DecoderOption[notificationSettings] {
	options := []DecoderOption[notificationSettings]{}

	for _, optName := range tc.Options {
		switch optName {
		case "use_number":
			options = append(options, WithUseNumber[notificationSettings]())
		case "disallow_unknown":
			options = append(options, WithDisallowUnknownFields[notificationSettings]())
		}
	}

	for _, hookName := range tc.PreHooks {
		switch hookName {
		case "quiet_hours_split":
			options = append(options, WithPreHook[notificationSettings](quietHoursPreHook))
		}
	}

	for _, hookName := range tc.PostHooks {
		switch hookName {
		case "ensure_tag":
			options = append(options, WithPostHook[notificationSettings](ensureTagPostHook))
		}
	}

	if tc.CustomDecoder != "" {
		switch tc.CustomDecoder {
		case "snapshot_string":
			options = append(options, WithCustomDecoder[notificationSettings](snapshotStringDecoder))
		}
	}

	return options
}

func quietHoursPreHook(_ Context, payload map[string]any) (map[string]any, error) {
	value, ok := payload["quietHours"].(string)
	if !ok || value == "" {
		return payload, nil
	}

	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid quiet hours payload %q", value)
	}

	payload["quietHours"] = map[string]any{
		"start": strings.TrimSpace(parts[0]),
		"end":   strings.TrimSpace(parts[1]),
	}
	return payload, nil
}

func ensureTagPostHook(ctx Context, snapshot *notificationSettings) error {
	if snapshot == nil {
		return errors.New("snapshot is nil")
	}
	if len(snapshot.Tags) > 0 {
		return nil
	}
	identifier := slugIdentifier(ctx.Slug)
	snapshot.Tags = []string{fmt.Sprintf("%s:%s", ctx.Scope, identifier)}
	return nil
}

func snapshotStringDecoder(ctx Context, payload map[string]any) (notificationSettings, error) {
	var zero notificationSettings
	raw, ok := payload["snapshot"].(string)
	if !ok || raw == "" {
		return zero, fmt.Errorf("missing snapshot string for slug %q", ctx.Slug)
	}
	var out notificationSettings
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return zero, err
	}
	return out, nil
}

func slugIdentifier(slug string) string {
	if slug == "" {
		return ""
	}
	parts := strings.Split(slug, "/")
	if len(parts) < 2 {
		return slug
	}
	return parts[1]
}

type fixture struct {
	Description string        `json:"description"`
	Cases       []fixtureCase `json:"cases"`
}

type fixtureCase struct {
	Name          string               `json:"name"`
	Slug          string               `json:"slug"`
	Scope         string               `json:"scope"`
	Input         map[string]any       `json:"input"`
	Expect        notificationSettings `json:"expect"`
	ExpectErr     string               `json:"expectErr"`
	PreHooks      []string             `json:"preHooks"`
	PostHooks     []string             `json:"postHooks"`
	Options       []string             `json:"options"`
	CustomDecoder string               `json:"customDecoder"`
}

type notificationSettings struct {
	Enabled    bool            `json:"enabled"`
	QuietHours quietHours      `json:"quietHours"`
	Channels   channelSettings `json:"channels"`
	Limits     limits          `json:"limits"`
	Tags       []string        `json:"tags"`
}

type quietHours struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type channelSettings struct {
	Email channel `json:"email"`
	Push  channel `json:"push"`
}

type channel struct {
	Enabled   bool   `json:"enabled"`
	Frequency string `json:"frequency"`
	Threshold int    `json:"threshold"`
}

type limits struct {
	Daily   int `json:"daily"`
	Monthly int `json:"monthly"`
}

func loadFixture(t *testing.T, name string) fixture {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read hydrate fixture %q: %v", name, err)
	}
	var fx fixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("failed to unmarshal hydrate fixture %q: %v", name, err)
	}
	return fx
}
