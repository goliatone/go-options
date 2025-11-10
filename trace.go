package opts

import (
	"encoding/json"
)

// Trace captures provenance information for a given path lookup across the
// scoped layers that produced the effective value.
type Trace struct {
	Path   string       `json:"path"`
	Layers []Provenance `json:"layers"`
}

// Provenance details how a specific scope contributed to a traced path.
type Provenance struct {
	Scope      Scope  `json:"scope"`
	SnapshotID string `json:"snapshot_id,omitempty"`
	Path       string `json:"path"`
	Value      any    `json:"value,omitempty"`
	Found      bool   `json:"found"`
}

// ToJSON serialises the trace into JSON for logging or transport helpers.
func (t Trace) ToJSON() ([]byte, error) {
	type alias Trace
	return json.Marshal(alias(t))
}

// TraceFromJSON deserialises a JSON payload that was previously generated via
// ToJSON.
func TraceFromJSON(payload []byte) (Trace, error) {
	type alias Trace
	var trace alias
	if err := json.Unmarshal(payload, &trace); err != nil {
		return Trace{}, err
	}
	return Trace(trace), nil
}
