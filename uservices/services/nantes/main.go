// Command nantes is the "nantes" microservice: it caches recent hand scores keyed by card codes.
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

const serviceName = "nantes"

const serviceVersion = "1.0.0"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	svc.Handle("/version", handleVersion(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// cacheState memoizes hand scores keyed by the concatenated card codes.
type cacheState struct {
	mu     sync.Mutex
	scores map[string]cribbage.ScoreBreakdown
}

var cache = cacheState{scores: map[string]cribbage.ScoreBreakdown{}}

// cacheRequest carries a hand and its already-computed breakdown to store.
type cacheRequest struct {
	Hand      cribbage.Hand           `json:"hand"`
	Breakdown cribbage.ScoreBreakdown `json:"breakdown"`
}

// key derives the cache key from a hand's canonical card codes.
func key(hand cribbage.Hand) string {
	k := ""
	for _, c := range hand.AllCards() {
		k += c.Code()
	}
	return k
}

// handleScore returns a cached breakdown on hit, or stores the supplied one on
// miss.
func handleScore(_ *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req cacheRequest
		if err := service.DecodeJSON(r, &req); err != nil {
			service.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		k := key(req.Hand)
		cache.mu.Lock()
		defer cache.mu.Unlock()
		if bd, ok := cache.scores[k]; ok {
			service.WriteJSON(w, http.StatusOK, map[string]any{"hit": true, "breakdown": bd})
			return
		}
		cache.scores[k] = req.Breakdown
		service.WriteJSON(w, http.StatusOK, map[string]any{"hit": false, "breakdown": req.Breakdown})
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
