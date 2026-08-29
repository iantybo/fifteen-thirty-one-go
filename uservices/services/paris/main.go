// Command paris is the "paris" microservice: it tracks high scores across recently scored hands.
//
// Tier and Argo-rollout status come from the shared catalog (see
// pkg/catalog); this service is tier-4 (impact tier 4 of 4) and deploys via a plain Kubernetes Deployment. It builds on the shared service
// scaffold and cribbage domain library and is a leaf service that performs its work locally.
package main

import (
	"net/http"

	"fifteen-thirty-one-go/uservices/pkg/cribbage"
	"fifteen-thirty-one-go/uservices/pkg/service"
	"sort"
	"sync"
)

const serviceName = "paris"

const serviceVersion = "1.0.0"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	svc.Handle("/version", handleVersion(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// entry is a single leaderboard record.
type entry struct {
	Label string `json:"label"`
	Total int    `json:"total"`
}

// leaderboardState keeps the top scores seen so far.
type leaderboardState struct {
	mu  sync.Mutex
	top []entry
}

var board leaderboardState

const maxEntries = 5

// scoreSubmission carries a labeled breakdown to record on the leaderboard.
type scoreSubmission struct {
	Label     string                  `json:"label"`
	Breakdown cribbage.ScoreBreakdown `json:"breakdown"`
}

// handleScore inserts a submission's total into the sorted top-N leaderboard.
func handleScore(_ *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var sub scoreSubmission
		if err := service.DecodeJSON(r, &sub); err != nil {
			service.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		board.mu.Lock()
		defer board.mu.Unlock()
		board.top = append(board.top, entry{Label: sub.Label, Total: sub.Breakdown.Total()})
		sort.Slice(board.top, func(i, j int) bool { return board.top[i].Total > board.top[j].Total })
		if len(board.top) > maxEntries {
			board.top = board.top[:maxEntries]
		}
		snapshot := make([]entry, len(board.top))
		copy(snapshot, board.top)
		service.WriteJSON(w, http.StatusOK, map[string]any{"leaderboard": snapshot})
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
