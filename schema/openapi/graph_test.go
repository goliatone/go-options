package openapi

import (
	"reflect"
	"testing"
)

func TestBuildSchemaGraphMetadata(t *testing.T) {
	type Credentials struct {
		Username string `json:"username" default:"admin" minLength:"3" maxLength:"64" pattern:"^[a-zA-Z0-9_]+$" formgen:"label=Username,placeholder=Enter user"`
		Password string `json:"password,omitempty" formgen:"widget=password" minLength:"8"`
	}
	type Service struct {
		Host         string        `json:"host" default:"localhost" minLength:"3" formgen:"label=Host,placeholder=example.com"`
		Port         int           `json:"port" minimum:"1" maximum:"65535" default:"443" enum:"80,443" formgen:"label=Port"`
		Enabled      *bool         `json:"enabled,omitempty" default:"true"`
		Mode         string        `json:"mode,omitempty" enum:"active,passive" relationship:"type=belongsTo,target=#/components/schemas/Mode"`
		Credentials  Credentials   `json:"credentials"`
		Dependencies []Credentials `json:"dependencies"`
	}

	node, err := buildSchemaGraph(Service{})
	if err != nil {
		t.Fatalf("buildSchemaGraph returned error: %v", err)
	}

	schema := node.inlineOpenAPI()
	if schema["type"] != "object" {
		t.Fatalf("expected object type, got %v", schema["type"])
	}
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("expected required slice, got %T", schema["required"])
	}
	expectedRequired := []string{"credentials", "dependencies", "host", "port"}
	if !reflect.DeepEqual(expectedRequired, required) {
		t.Fatalf("unexpected required fields\nwant: %v\ngot:  %v", expectedRequired, required)
	}

	props := schema["properties"].(map[string]any)
	host := props["host"].(map[string]any)
	if host["default"] != "localhost" {
		t.Fatalf("expected host default localhost, got %v", host["default"])
	}
	if host["minLength"].(int) != 3 {
		t.Fatalf("expected host minLength 3, got %v", host["minLength"])
	}
	formgen := host["x-formgen"].(map[string]any)
	if formgen["label"] != "Host" {
		t.Fatalf("expected host formgen label, got %v", formgen["label"])
	}
	if formgen["placeholder"] != "example.com" {
		t.Fatalf("expected host placeholder example.com, got %v", formgen["placeholder"])
	}

	port := props["port"].(map[string]any)
	if port["minimum"].(float64) != 1 {
		t.Fatalf("expected port minimum 1, got %v", port["minimum"])
	}
	if port["maximum"].(float64) != 65535 {
		t.Fatalf("expected port maximum 65535, got %v", port["maximum"])
	}
	if port["default"] != int64(443) {
		t.Fatalf("expected port default 443, got %v", port["default"])
	}
	enum := port["enum"].([]any)
	if len(enum) != 2 || enum[0] != int64(80) || enum[1] != int64(443) {
		t.Fatalf("unexpected port enum %v", enum)
	}

	mode := props["mode"].(map[string]any)
	relationships := mode["x-relationships"].(map[string]any)
	if relationships["type"] != "belongsTo" {
		t.Fatalf("expected relationship type belongsTo, got %v", relationships["type"])
	}
	if relationships["target"] != "#/components/schemas/Mode" {
		t.Fatalf("expected relationship target, got %v", relationships["target"])
	}

	credentials := props["credentials"].(map[string]any)
	if _, exists := credentials["required"]; !exists {
		t.Fatalf("expected credentials required metadata")
	}
	deps := props["dependencies"].(map[string]any)
	items := deps["items"].(map[string]any)
	if items["type"] != "object" {
		t.Fatalf("expected array items object type, got %v", items["type"])
	}
}

func TestSchemaNodeDigest(t *testing.T) {
	type A struct {
		Value string `json:"value" minLength:"3"`
	}
	type B struct {
		Value string `json:"value" minLength:"4"`
	}

	nodeA1, err := buildSchemaGraph(A{})
	if err != nil {
		t.Fatalf("buildSchemaGraph(A) error: %v", err)
	}
	nodeA2, err := buildSchemaGraph(A{})
	if err != nil {
		t.Fatalf("buildSchemaGraph(A) second error: %v", err)
	}
	if nodeA1.Digest() != nodeA2.Digest() {
		t.Fatalf("expected identical digests for equivalent schemas")
	}

	nodeB, err := buildSchemaGraph(B{})
	if err != nil {
		t.Fatalf("buildSchemaGraph(B) error: %v", err)
	}
	if nodeA1.Digest() == nodeB.Digest() {
		t.Fatalf("expected differing digests for differing schemas")
	}
}
