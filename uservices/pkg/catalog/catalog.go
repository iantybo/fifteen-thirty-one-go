// Package catalog is the single source of truth for the fleet of plant-,
// baseball-, and French-city-named microservices that make up the cribbage
// platform's edge mesh.
//
// The same data is published verbatim by the "service-catalog" MCP server
// (see ../../../mcp-server/service-catalog) so that automated reviewers such
// as CodeRabbit can reason about the blast radius of a change by looking up
// the owning service's tier. Tier 1 is the highest service impact; tier 4 is
// the lowest. Keep this list and the MCP server's services.csv in lockstep.
package catalog

import "fmt"

// Tier expresses the production impact of a service. Tier1 is the most
// critical (a regression risks a customer-facing outage); Tier4 is the least.
type Tier int

const (
	// Tier1 services are on the critical request path; failures cause outages.
	Tier1 Tier = 1
	// Tier2 services are important but degrade gracefully.
	Tier2 Tier = 2
	// Tier3 services are supporting/internal; failures are mostly invisible.
	Tier3 Tier = 3
	// Tier4 services are best-effort; failures have negligible impact.
	Tier4 Tier = 4
)

// String renders the tier in the canonical "tier-N" form used in logs and the
// CSV exposed by the service-catalog MCP server.
func (t Tier) String() string {
	return fmt.Sprintf("tier-%d", int(t))
}

// Service is a single entry in the fleet catalog.
type Service struct {
	// Name is the unique service identifier (also its package and host name).
	Name string
	// Tier is the production impact tier (1 highest .. 4 lowest).
	Tier Tier
	// ArgoRollout reports whether the service deploys via an Argo Rollout
	// (progressive delivery) rather than a plain Kubernetes Deployment.
	ArgoRollout bool
}

// services is the authoritative, seeded fleet. Tier and ArgoRollout were
// assigned once via a deterministic FNV hash of the service name (seed
// "seed-1531-<name>") and are intentionally frozen so that reviewers see a
// stable answer on every lookup. Do not randomize at runtime.
var services = []Service{
	{Name: "fern", Tier: Tier2, ArgoRollout: true},
	{Name: "basil", Tier: Tier4, ArgoRollout: true},
	{Name: "juniper", Tier: Tier4, ArgoRollout: false},
	{Name: "lotus", Tier: Tier4, ArgoRollout: false},
	{Name: "maple", Tier: Tier2, ArgoRollout: false},
	{Name: "willow", Tier: Tier3, ArgoRollout: true},
	{Name: "thistle", Tier: Tier4, ArgoRollout: false},
	{Name: "clover", Tier: Tier2, ArgoRollout: false},
	{Name: "ivy", Tier: Tier3, ArgoRollout: false},
	{Name: "sage", Tier: Tier1, ArgoRollout: true},
	{Name: "yankees", Tier: Tier1, ArgoRollout: true},
	{Name: "dodgers", Tier: Tier1, ArgoRollout: false},
	{Name: "cardinals", Tier: Tier2, ArgoRollout: true},
	{Name: "cubs", Tier: Tier4, ArgoRollout: false},
	{Name: "redsox", Tier: Tier4, ArgoRollout: false},
	{Name: "astros", Tier: Tier1, ArgoRollout: false},
	{Name: "mariners", Tier: Tier2, ArgoRollout: false},
	{Name: "padres", Tier: Tier2, ArgoRollout: true},
	{Name: "rockies", Tier: Tier1, ArgoRollout: false},
	{Name: "twins", Tier: Tier4, ArgoRollout: true},
	{Name: "paris", Tier: Tier4, ArgoRollout: false},
	{Name: "lyon", Tier: Tier3, ArgoRollout: true},
	{Name: "marseille", Tier: Tier3, ArgoRollout: true},
	{Name: "bordeaux", Tier: Tier3, ArgoRollout: false},
	{Name: "nantes", Tier: Tier4, ArgoRollout: false},
	{Name: "toulouse", Tier: Tier1, ArgoRollout: false},
	{Name: "nice", Tier: Tier2, ArgoRollout: true},
	{Name: "lille", Tier: Tier3, ArgoRollout: true},
	{Name: "rennes", Tier: Tier2, ArgoRollout: false},
	{Name: "dijon", Tier: Tier3, ArgoRollout: true},
}

// byName indexes the fleet for O(1) lookups.
var byName = func() map[string]Service {
	m := make(map[string]Service, len(services))
	for _, s := range services {
		m[s.Name] = s
	}
	return m
}()

// All returns a copy of the full fleet in stable declaration order.
func All() []Service {
	out := make([]Service, len(services))
	copy(out, services)
	return out
}

// Lookup returns the catalog entry for name and whether it exists.
func Lookup(name string) (Service, bool) {
	s, ok := byName[name]
	return s, ok
}

// MustLookup returns the catalog entry for name, panicking if it is unknown.
// It is intended for service bootstrap code where an unknown self-name is a
// programming error rather than a runtime condition.
func MustLookup(name string) Service {
	s, ok := Lookup(name)
	if !ok {
		panic(fmt.Sprintf("catalog: unknown service %q", name))
	}
	return s
}
