package opts

const (
	// Recommended priorities for common layering patterns. Higher numbers win.
	ScopePrioritySystem = 100
	ScopePriorityTenant = 200
	ScopePriorityOrg    = 300
	ScopePriorityTeam   = 400
	ScopePriorityUser   = 500
)

// SystemTenantOrgTeamUser assembles a canonical five-layer stack (system →
// tenant → org → team → user) and returns the merged options wrapper.
func SystemTenantOrgTeamUser[T any](system, tenant, org, team, user T) (*Options[T], error) {
	layers := []Layer[T]{
		NewLayer(NewScope("user", ScopePriorityUser, WithScopeLabel("User")), user),
		NewLayer(NewScope("team", ScopePriorityTeam, WithScopeLabel("Team")), team),
		NewLayer(NewScope("org", ScopePriorityOrg, WithScopeLabel("Organization")), org),
		NewLayer(NewScope("tenant", ScopePriorityTenant, WithScopeLabel("Tenant")), tenant),
		NewLayer(NewScope("system", ScopePrioritySystem, WithScopeLabel("System Defaults")), system),
	}
	stack, err := NewStack(layers...)
	if err != nil {
		return nil, err
	}
	return stack.Merge()
}
