// Command thistle is the "thistle" microservice: it formats a score breakdown into a human-readable summary.
//
// Tier and Argo-rollout status come from the shared catalog (see
// pkg/catalog); this service is tier-4 (impact tier 4 of 4) and deploys via a plain Kubernetes Deployment. It builds on the shared service
// scaffold and cribbage domain library and is a leaf service that performs its work locally.
package main

import (
	"net/http"

	"fifteen-thirty-one-go/uservices/pkg/cribbage"
	"fifteen-thirty-one-go/uservices/pkg/service"
	"fmt"
)

const serviceName = "thistle"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// handleScore formats a score breakdown into a human-readable, line-per-rule
// summary plus the grand total.
func handleScore(_ *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var bd cribbage.ScoreBreakdown
		if err := service.DecodeJSON(r, &bd); err != nil {
			service.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		lines := make([]string, 0, len(bd.Components))
		for _, c := range bd.Components {
			lines = append(lines, formatComponent(c))
		}
		service.WriteJSON(w, http.StatusOK, map[string]any{
			"lines": lines,
			"total": bd.Total(),
		})
	}
}

// formatComponent renders a single score component as "rule: N points (detail)".
func formatComponent(c cribbage.ScoreComponent) string {
	line := fmt.Sprintf("%s: %d points", c.Rule, c.Points)
	if c.Detail != "" {
		line += " (" + c.Detail + ")"
	}
	return line
}
