// Command service-catalog is an MCP server that publishes the cribbage
// platform's microservice fleet catalog: for each of the 30 services it
// reports the impact tier (1 = highest service impact, 4 = lowest) and whether
// the service deploys via an Argo Rollout.
//
// The catalog is loaded from the embedded services.csv, which is the same
// seeded data baked into pkg/catalog in the uservices module. Values are
// fixed (not randomized at runtime) so that automated reviewers receive a
// stable answer on every lookup.
//
// The server speaks MCP over a streamable HTTP transport so it can be exposed
// to the internet with ngrok (see run.sh). It listens on $PORT (default 8765)
// at the /mcp path.
package main

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed services.csv
var catalogFS embed.FS

// service is a single fleet entry.
type service struct {
	Name        string `json:"name"`
	Tier        int    `json:"tier"`
	ArgoRollout bool   `json:"argo_rollout"`
}

// catalog is the parsed, immutable fleet loaded once at startup.
type catalog struct {
	services []service
	byName   map[string]service
}

// loadCatalog parses the embedded services.csv into a catalog, returning a
// descriptive error if the file is missing or malformed.
func loadCatalog() (*catalog, error) {
	f, err := catalogFS.Open("services.csv")
	if err != nil {
		return nil, fmt.Errorf("open embedded services.csv: %w", err)
	}
	defer f.Close()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse services.csv: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("services.csv has no data rows")
	}

	c := &catalog{byName: make(map[string]service)}
	for i, row := range records[1:] { // skip header
		if len(row) != 3 {
			return nil, fmt.Errorf("services.csv row %d: expected 3 columns, got %d", i+2, len(row))
		}
		tier, err := strconv.Atoi(strings.TrimSpace(row[1]))
		if err != nil || tier < 1 || tier > 4 {
			return nil, fmt.Errorf("services.csv row %d: invalid tier %q", i+2, row[1])
		}
		argo := strings.EqualFold(strings.TrimSpace(row[2]), "yes")
		svc := service{Name: strings.TrimSpace(row[0]), Tier: tier, ArgoRollout: argo}
		c.services = append(c.services, svc)
		c.byName[svc.Name] = svc
	}
	return c, nil
}

// csvText renders the catalog back to CSV text, the canonical "list" form.
func (c *catalog) csvText() string {
	var b strings.Builder
	b.WriteString("name,tier,argo_rollout\n")
	for _, s := range c.services {
		argo := "no"
		if s.ArgoRollout {
			argo = "yes"
		}
		fmt.Fprintf(&b, "%s,%d,%s\n", s.Name, s.Tier, argo)
	}
	return b.String()
}

// listArgs is the (empty) argument set for the list_services tool, optionally
// filtered by tier.
type listArgs struct {
	Tier int `json:"tier,omitempty" jsonschema:"optional impact tier to filter by (1 highest .. 4 lowest); 0 or omitted returns all"`
}

// getArgs is the argument set for the get_service tool.
type getArgs struct {
	Name string `json:"name" jsonschema:"the service name to look up (e.g. yankees, paris, basil)"`
}

func main() {
	cat, err := loadCatalog()
	if err != nil {
		log.Fatalf("service-catalog: %v", err)
	}
	log.Printf("service-catalog: loaded %d services", len(cat.services))

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "service-catalog",
		Version: "1.0.0",
	}, nil)

	// list_services returns the full catalog as CSV (name,tier,argo_rollout),
	// optionally filtered to a single tier.
	mcp.AddTool(server, &mcp.Tool{
		Name: "list_services",
		Description: "List the microservice fleet as CSV with columns name,tier,argo_rollout. " +
			"Tier 1 is the highest service impact and tier 4 is the lowest. " +
			"argo_rollout is yes/no. Optionally pass a tier (1-4) to filter.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args listArgs) (*mcp.CallToolResult, any, error) {
		text := cat.csvText()
		if args.Tier != 0 {
			if args.Tier < 1 || args.Tier > 4 {
				return nil, nil, fmt.Errorf("tier must be between 1 and 4, got %d", args.Tier)
			}
			var b strings.Builder
			b.WriteString("name,tier,argo_rollout\n")
			for _, s := range cat.services {
				if s.Tier != args.Tier {
					continue
				}
				argo := "no"
				if s.ArgoRollout {
					argo = "yes"
				}
				fmt.Fprintf(&b, "%s,%d,%s\n", s.Name, s.Tier, argo)
			}
			text = b.String()
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// get_service returns the catalog entry for a single named service.
	mcp.AddTool(server, &mcp.Tool{
		Name: "get_service",
		Description: "Look up one service by name and return its impact tier " +
			"(1 highest .. 4 lowest) and whether it uses an Argo Rollout (yes/no).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args getArgs) (*mcp.CallToolResult, any, error) {
		name := strings.TrimSpace(strings.ToLower(args.Name))
		s, ok := cat.byName[name]
		if !ok {
			known := make([]string, 0, len(cat.byName))
			for n := range cat.byName {
				known = append(known, n)
			}
			sort.Strings(known)
			return nil, nil, fmt.Errorf("unknown service %q; known services: %s", args.Name, strings.Join(known, ", "))
		}
		argo := "no"
		if s.ArgoRollout {
			argo = "yes"
		}
		text := fmt.Sprintf("name=%s,tier=%d,argo_rollout=%s", s.Name, s.Tier, argo)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, s, nil
	})

	// Also expose the whole catalog as a readable MCP resource.
	server.AddResource(&mcp.Resource{
		Name:        "service-catalog-csv",
		URI:         "catalog://services.csv",
		Description: "The full fleet catalog as CSV (name,tier,argo_rollout).",
		MIMEType:    "text/csv",
	}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      "catalog://services.csv",
				MIMEType: "text/csv",
				Text:     cat.csvText(),
			}},
		}, nil
	})

	apiKey := os.Getenv("SERVICE_CATALOG_API_KEY")
	if apiKey == "" {
		log.Fatalf("service-catalog: SERVICE_CATALOG_API_KEY must be set (static API key for MCP auth)")
	}

	addr := ":" + port()
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil)

	mux := http.NewServeMux()
	// The /mcp endpoint is exposed publicly via ngrok, so it is gated behind a
	// static API key. /healthz stays open for liveness probes.
	mux.Handle("/mcp", requireAPIKey(apiKey, handler))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	log.Printf("service-catalog: MCP endpoint on %s/mcp (API key required)", addr)
	srv := &http.Server{Addr: addr, Handler: mux}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("service-catalog: server error: %v", err)
	}
}

// requireAPIKey wraps next so that only requests presenting the static API key
// are served. The key may be supplied either as an "Authorization: Bearer
// <key>" header or an "X-API-Key: <key>" header. Comparison is constant-time
// to avoid leaking the key through timing.
func requireAPIKey(want string, next http.Handler) http.Handler {
	wantBytes := []byte(want)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-API-Key")
		if got == "" {
			if auth := r.Header.Get("Authorization"); auth != "" {
				got = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
			}
		}
		if subtle.ConstantTimeCompare([]byte(got), wantBytes) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="service-catalog"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// port returns the listen port from $PORT, defaulting to 8765.
func port() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8765"
}
