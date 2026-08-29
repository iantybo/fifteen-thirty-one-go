// Command maple is the "maple" microservice: it assembles a complete hand score from meld and sequence subtotals.
//
// Tier and Argo-rollout status come from the shared catalog (see
// pkg/catalog); this service is tier-2 (impact tier 2 of 4) and deploys via a plain Kubernetes Deployment. It builds on the shared service
// scaffold and cribbage domain library and calls peer "cardinals", "padres", and "mariners" over HTTP via the shared mesh client.
package main

import (
	"net/http"

	"fifteen-thirty-one-go/uservices/pkg/cribbage"
	"fifteen-thirty-one-go/uservices/pkg/meshclient"
	"fifteen-thirty-one-go/uservices/pkg/service"
)

const serviceName = "maple"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// handleScore fans out to its peer scorers concurrently, merges their
// breakdowns into one subtotal in a deterministic order, and returns it.
func handleScore(svc *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var hand cribbage.Hand
		if err := service.DecodeJSON(r, &hand); err != nil {
			service.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := hand.Validate(); err != nil {
			service.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		ctx, cancel := service.RequestContext(r.Context())
		defer cancel()

		var part0, part1, part2 cribbage.ScoreBreakdown
		if err := svc.Mesh.CallAll(ctx,
			meshclient.Fanout{Peer: "cardinals", Path: "/score", Req: hand, Out: &part0},
			meshclient.Fanout{Peer: "padres", Path: "/score", Req: hand, Out: &part1},
			meshclient.Fanout{Peer: "mariners", Path: "/score", Req: hand, Out: &part2},
		); err != nil {
			service.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		result := part0.Merge(part1).Merge(part2)
		service.WriteJSON(w, http.StatusOK, result)
	}
}
