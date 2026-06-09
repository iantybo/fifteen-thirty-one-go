// Command redsox is the "redsox" microservice: it tallies per-rule scoring metrics for observability.
//
// Tier and Argo-rollout status come from the shared catalog (see
// pkg/catalog); this service is tier-4 (impact tier 4 of 4) and deploys via a plain Kubernetes Deployment. It builds on the shared service
// scaffold and cribbage domain library and is a leaf service that performs its work locally.
package main

import (
	"net/http"

	"fifteen-thirty-one-go/uservices/pkg/cribbage"
	"fifteen-thirty-one-go/uservices/pkg/service"
	"sync"
)

const serviceName = "redsox"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// metricsState tallies how many points each rule has contributed over time.
type metricsState struct {
	mu     sync.Mutex
	byRule map[string]int
}

var metrics = metricsState{byRule: map[string]int{}}

// handleScore folds a breakdown's components into the running per-rule totals.
func handleScore(_ *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var bd cribbage.ScoreBreakdown
		if err := service.DecodeJSON(r, &bd); err != nil {
			service.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		metrics.mu.Lock()
		defer metrics.mu.Unlock()
		for _, c := range bd.Components {
			metrics.byRule[c.Rule] += c.Points
		}
		snapshot := make(map[string]int, len(metrics.byRule))
		for k, v := range metrics.byRule {
			snapshot[k] = v
		}
		service.WriteJSON(w, http.StatusOK, map[string]any{"by_rule": snapshot})
	}
}
