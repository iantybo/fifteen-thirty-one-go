// Command willow is the "willow" microservice: it scores two hands and reports which is higher.
//
// Tier and Argo-rollout status come from the shared catalog (see
// pkg/catalog); this service is tier-3 (impact tier 3 of 4) and deploys via an Argo Rollout. It builds on the shared service
// scaffold and cribbage domain library and calls peer "maple" and "ivy" over HTTP via the shared mesh client.
package main

import (
	"net/http"

	"fifteen-thirty-one-go/uservices/pkg/cribbage"
	"fifteen-thirty-one-go/uservices/pkg/service"
)

const serviceName = "willow"

const serviceVersion = "1.0.0"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	svc.Handle("/version", handleVersion(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// comparison is the request shape: two hands to score and rank.
type comparison struct {
	A cribbage.Hand `json:"a"`
	B cribbage.Hand `json:"b"`
}

// handleScore scores both hands via the aggregators and reports the winner.
func handleScore(svc *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var cmp comparison
		if err := service.DecodeJSON(r, &cmp); err != nil {
			service.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		ctx, cancel := service.RequestContext(r.Context())
		defer cancel()

		var a, b cribbage.ScoreBreakdown
		if err := svc.Mesh.Call(ctx, "maple", "/score", cmp.A, &a); err != nil {
			service.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		if err := svc.Mesh.Call(ctx, "ivy", "/score", cmp.B, &b); err != nil {
			service.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		winner := "tie"
		switch {
		case a.Total() > b.Total():
			winner = "a"
		case b.Total() > a.Total():
			winner = "b"
		}
		service.WriteJSON(w, http.StatusOK, map[string]any{
			"a_total": a.Total(),
			"b_total": b.Total(),
			"winner":  winner,
		})
	}
}

// handleVersion reports the service's name and build version.
func handleVersion(svc *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		service.WriteJSON(w, http.StatusOK, map[string]string{
			"service": svc.Self.Name,
			"version": serviceVersion,
		})
	}
}
