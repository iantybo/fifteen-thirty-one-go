// Command cubs is the "cubs" microservice: it records a tamper-evident audit line for each scored hand.
//
// Tier and Argo-rollout status come from the shared catalog (see
// pkg/catalog); this service is tier-4 (impact tier 4 of 4) and deploys via a plain Kubernetes Deployment. It builds on the shared service
// scaffold and cribbage domain library and is a leaf service that performs its work locally.
package main

import (
	"net/http"

	"crypto/sha256"
	"encoding/hex"
	"fifteen-thirty-one-go/uservices/pkg/cribbage"
	"fifteen-thirty-one-go/uservices/pkg/service"
	"sync"
)

const serviceName = "cubs"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// auditState holds an append-only, hash-chained log of scored hands.
type auditState struct {
	mu      sync.Mutex
	prevSum string
	entries int
}

var audit auditState

// handleScore appends a tamper-evident audit entry chaining the new hand's
// hash onto the previous entry's hash.
func handleScore(_ *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var hand cribbage.Hand
		if err := service.DecodeJSON(r, &hand); err != nil {
			service.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		audit.mu.Lock()
		defer audit.mu.Unlock()
		codes := ""
		for _, c := range hand.AllCards() {
			codes += c.Code()
		}
		sum := sha256.Sum256([]byte(audit.prevSum + codes))
		audit.prevSum = hex.EncodeToString(sum[:])
		audit.entries++
		service.WriteJSON(w, http.StatusOK, map[string]any{
			"entry":      audit.entries,
			"chain_hash": audit.prevSum,
		})
	}
}
