// Package service is the shared scaffold that every microservice's main.go
// builds on. It wires up a service's catalog identity, an HTTP mux with a
// standard /health endpoint, and a meshclient for talking to peers, so each
// service package only has to supply its own domain handler.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"fifteen-thirty-one-go/uservices/pkg/catalog"
	"fifteen-thirty-one-go/uservices/pkg/meshclient"
)

// Service bundles a catalog identity with the shared mesh client.
type Service struct {
	Self catalog.Service
	Mesh *meshclient.Client
	mux  *http.ServeMux
}

// New constructs a Service for the catalog entry named name. It panics if name
// is not in the catalog, since that is a build-time wiring error.
func New(name string) *Service {
	self := catalog.MustLookup(name)
	s := &Service{
		Self: self,
		Mesh: meshclient.New(nil),
		mux:  http.NewServeMux(),
	}
	s.mux.HandleFunc("/health", s.handleHealth)
	return s
}

// Handle registers an HTTP handler for the service's domain endpoint.
func (s *Service) Handle(path string, h http.HandlerFunc) {
	s.mux.HandleFunc(path, h)
}

// handleHealth reports the service's identity and tier so operators (and the
// service-catalog MCP server) can confirm what is running.
func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]any{
		"service":      s.Self.Name,
		"tier":         s.Self.Tier.String(),
		"argo_rollout": s.Self.ArgoRollout,
		"status":       "ok",
	})
}

// ListenAndServe starts the HTTP server on addr with sane timeouts and logs
// the service's tier on startup.
func (s *Service) ListenAndServe(addr string) error {
	log.Printf("starting %s (%s, argo_rollout=%t) on %s",
		s.Self.Name, s.Self.Tier, s.Self.ArgoRollout, addr)

	srv := &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return srv.ListenAndServe()
}

// maxRequestBody bounds how much of a request body DecodeJSON will read, so a
// malformed or hostile client cannot exhaust memory on a decode.
const maxRequestBody = 1 << 20 // 1 MiB

// ErrDecodeBody wraps every failure to read or decode a request body, so
// callers can classify a bad request with errors.Is rather than by message.
var ErrDecodeBody = errors.New("service: decode request body")

// DecodeJSON reads and decodes a single JSON value from the request body into
// v, returning a descriptive error on failure. Bodies larger than
// maxRequestBody are rejected outright rather than silently truncated, and any
// data trailing the first JSON value is an error, so a request cannot smuggle
// extra bytes past the limit.
func DecodeJSON(r *http.Request, v any) (err error) {
	defer func() {
		if closeErr := r.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("service: close request body: %w", closeErr))
		}
	}()

	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxRequestBody))
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: %w", ErrDecodeBody, err)
	}
	switch err := dec.Decode(&struct{}{}); {
	case errors.Is(err, io.EOF):
		return nil
	case err != nil:
		return fmt.Errorf("%w: read after JSON value: %w", ErrDecodeBody, err)
	default:
		return fmt.Errorf("%w: unexpected data after JSON value", ErrDecodeBody)
	}
}

// WriteJSON writes v as a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("service: encode response: %v", err)
	}
}

// RequestContext returns a child context bounded by a per-request deadline,
// used when fanning out to peers.
func RequestContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 4*time.Second)
}
