// Command lotus is the "lotus" microservice: it parses raw card-code strings into structured cards.
//
// Tier and Argo-rollout status come from the shared catalog (see
// pkg/catalog); this service is tier-4 (impact tier 4 of 4) and deploys via a plain Kubernetes Deployment. It builds on the shared service
// scaffold and cribbage domain library and is a leaf service that performs its work locally.
package main

import (
	"net/http"

	"fifteen-thirty-one-go/uservices/pkg/cribbage"
	"fifteen-thirty-one-go/uservices/pkg/service"
)

const serviceName = "lotus"

const serviceVersion = "1.0.0"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	svc.Handle("/version", handleVersion(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// parseRequest carries raw card-code strings to be parsed into structured cards.
type parseRequest struct {
	Codes []string `json:"codes"`
}

// handleScore parses each raw card code into a structured card, rejecting the
// whole request if any code is malformed.
func handleScore(_ *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req parseRequest
		if err := service.DecodeJSON(r, &req); err != nil {
			service.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		cards := make([]cribbage.Card, 0, len(req.Codes))
		for _, code := range req.Codes {
			card, err := cribbage.ParseCard(code)
			if err != nil {
				service.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			cards = append(cards, card)
		}
		service.WriteJSON(w, http.StatusOK, map[string]any{"cards": cards})
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
