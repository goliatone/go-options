package opts

import (
	"fmt"
	"slices"
)

// ScopeLevel identifies the precedence of a snapshot. Higher levels override
// lower levels when layering.
type ScopeLevel int

const (
	// ScopeLevelUnknown guards against misconfiguration so call sites can detect
	// missing metadata.
	ScopeLevelUnknown ScopeLevel = iota
	// ScopeLevelGlobal represents the weakest layer (system defaults).
	ScopeLevelGlobal
	// ScopeLevelGroup represents a group-level override (e.g., team, tenant).
	ScopeLevelGroup
	// ScopeLevelUser represents the strongest layer containing per-user data.
	ScopeLevelUser
)

func (l ScopeLevel) String() string {
	switch l {
	case ScopeLevelGlobal:
		return "global"
	case ScopeLevelGroup:
		return "group"
	case ScopeLevelUser:
		return "user"
	default:
		return "unknown"
	}
}

// ParseScopeLevel converts a string representation into the corresponding
// ScopeLevel. Returns ScopeLevelUnknown for unrecognised values.
func ParseScopeLevel(value string) ScopeLevel {
	switch value {
	case "global", "GLOBAL":
		return ScopeLevelGlobal
	case "group", "GROUP":
		return ScopeLevelGroup
	case "user", "USER":
		return ScopeLevelUser
	default:
		return ScopeLevelUnknown
	}
}

// Scope names a snapshot within a layering chain.
type Scope struct {
	Key   string     // logical key for the settings domain (e.g., "notifications")
	Level ScopeLevel // precedence category
	User  string     // user identifier when Level == ScopeLevelUser
	Group string     // group identifier when Level == ScopeLevelGroup
}

// Identifier returns a stable slug that CMS adapters can use when composing
// deterministic storage keys (e.g., "user/123/settings-key").
func (s Scope) Identifier() string {
	switch s.Level {
	case ScopeLevelUser:
		return fmt.Sprintf("user/%s/%s", s.User, s.Key)
	case ScopeLevelGroup:
		return fmt.Sprintf("group/%s/%s", s.Group, s.Key)
	case ScopeLevelGlobal:
		return fmt.Sprintf("global/%s", s.Key)
	default:
		return fmt.Sprintf("unknown/%s", s.Key)
	}
}

// ScopeChain describes the ordered layering sequence from strongest to weakest.
type ScopeChain struct {
	ordered []Scope
}

// NewScopeChain constructs a chain and deduplicates scopes using their
// Identifier. The resulting order always places stronger levels before weaker
// ones while keeping relative ordering for peers.
func NewScopeChain(scopes ...Scope) ScopeChain {
	filtered := make([]Scope, 0, len(scopes))
	seen := map[string]struct{}{}

	for _, scope := range scopes {
		if scope.Level == ScopeLevelUnknown {
			continue
		}
		id := scope.Identifier()
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		filtered = append(filtered, scope)
	}

	slices.SortStableFunc(filtered, func(a, b Scope) int {
		if a.Level == b.Level {
			return 0
		}
		if a.Level > b.Level {
			return -1
		}
		return 1
	})

	return ScopeChain{ordered: filtered}
}

// Ordered returns the layering sequence from strongest (index 0) to weakest.
func (c ScopeChain) Ordered() []Scope {
	out := make([]Scope, len(c.ordered))
	copy(out, c.ordered)
	return out
}

// Strongest returns the first scope in the chain (zero scope if empty).
func (c ScopeChain) Strongest() Scope {
	if len(c.ordered) == 0 {
		return Scope{}
	}
	return c.ordered[0]
}

// Weakest returns the final scope in the chain (zero scope if empty).
func (c ScopeChain) Weakest() Scope {
	if len(c.ordered) == 0 {
		return Scope{}
	}
	return c.ordered[len(c.ordered)-1]
}
