// Command lyon is the "lyon" microservice: it evaluates candidate discards by scoring the resulting hand.
//
// Tier and Argo-rollout status come from the shared catalog (see
// pkg/catalog); this service is tier-3 (impact tier 3 of 4) and deploys via an Argo Rollout. It builds on the shared service
// scaffold and cribbage domain library and calls peer "maple" over HTTP via the shared mesh client.
package main

import (
	"net/http"

	"fifteen-thirty-one-go/uservices/pkg/cribbage"
	"fifteen-thirty-one-go/uservices/pkg/service"
)

const serviceName = "lyon"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// handleScore scores the candidate hand via the aggregator and labels the
// discard quality based on the resulting total.
func handleScore(svc *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var hand cribbage.Hand
		if err := service.DecodeJSON(r, &hand); err != nil {
			service.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		ctx, cancel := service.RequestContext(r.Context())
		defer cancel()

		var bd cribbage.ScoreBreakdown
		if err := svc.Mesh.Call(ctx, "maple", "/score", hand, &bd); err != nil {
			service.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		service.WriteJSON(w, http.StatusOK, map[string]any{
			"total":   bd.Total(),
			"quality": discardQuality(bd.Total()),
		})
	}
}

// discardQuality buckets a hand total into a qualitative label.
func discardQuality(total int) string {
	switch {
	case total >= 12:
		return "excellent"
	case total >= 8:
		return "good"
	case total >= 4:
		return "fair"
	default:
		return "poor"
	}
}
