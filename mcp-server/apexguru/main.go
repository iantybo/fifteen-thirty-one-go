// Command apexguru is a simulated ApexGuru MCP server for the CodeRabbit
// Salesforce proof of value.
//
// It stands in for the forcedotcom/sf-skills dx-apexguru-scan skill. That
// skill drives the real ApexGuru SFAP Scan API with an sf CLI org login, which
// CodeRabbit should never hold. Here an admin runs one MCP server inside their
// own boundary and registers it with CodeRabbit once. Every PR review and every
// local `coderabbit` CLI run can then call it without any developer
// configuring credentials.
//
// The tools are:
//   - apexguru_scan: static detection of the ApexGuru antipattern catalog,
//     enriched with projected impact at UAT or prod volumes.
//   - explain_soql: a simulated query plan across dev, uat, and prod.
//   - get_object_volumes: seeded sObject row counts and indexed fields.
//   - get_governor_limits: Apex per-transaction limits.
//   - get_runtime_hotspots: seeded debug-log telemetry per class.
//
// All org data is SEEDED (seed/*.json), so every review gets the same answer.
//
// The server speaks MCP over streamable HTTP at /mcp, behind a static API key,
// so it can be exposed with ngrok (see run.sh). It listens on $PORT (default
// 8766).
package main

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// maxRequestBytes bounds the HTTP body so oversized scans are rejected before
// JSON decoding. It leaves headroom over maxScanTotalBytes for JSON escaping.
const maxRequestBytes = 3 * maxScanTotalBytes

// textResult wraps markdown output as an MCP tool result.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

// newServer registers every tool against seed s.
func newServer(s *seedData) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "apexguru", Version: "1.0.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name: "apexguru_scan",
		Description: "Run an ApexGuru performance scan over Apex source (.cls/.trigger). " +
			"Detects SOQL in loops (direct and one hop through a called method), DML in loops, " +
			"Schema.getGlobalDescribe(), SOQL without WHERE or LIMIT, and unused SOQL fields. " +
			"Projects each finding against the chosen org's data volumes and runtime telemetry " +
			"(environment uat or prod gives 'Production insights'; dev gives 'Static only'). " +
			"Pass the full content of every changed Apex file.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args scanArgs) (*mcp.CallToolResult, any, error) {
		out, err := s.scan(args)
		if err != nil {
			return nil, nil, err
		}
		return textResult(out), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "explain_soql",
		Description: "Explain how a SOQL query will plan in dev, uat, and prod: object row counts, " +
			"index vs. table scan, estimated rows, and whether it breaks the 50,000-row limit or the " +
			"non-selective-query threshold in triggers. Use it on every SOQL statement in changed Apex " +
			"to catch queries that pass in dev but fail after promotion.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args explainArgs) (*mcp.CallToolResult, any, error) {
		out, err := s.explain(args)
		if err != nil {
			return nil, nil, err
		}
		return textResult(out), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_object_volumes",
		Description: "Return sObject row counts per org (dev, uat, prod) and each object's indexed fields.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args volumesArgs) (*mcp.CallToolResult, any, error) {
		out, err := s.volumes(args)
		if err != nil {
			return nil, nil, err
		}
		return textResult(out), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_governor_limits",
		Description: "Return the Apex per-transaction governor limits (synchronous and asynchronous).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		return textResult(governorLimits()), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_runtime_hotspots",
		Description: "Return runtime telemetry for an Apex class in uat or prod: CPU, heap, p95 SOQL and DML " +
			"usage against limits, call volume, and recent exceptions. Use it to judge how much limit " +
			"headroom a transaction has before a change adds more work to it.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args hotspotArgs) (*mcp.CallToolResult, any, error) {
		out, err := s.hotspots(args)
		if err != nil {
			return nil, nil, err
		}
		return textResult(out), nil, nil
	})
	return server
}

// defaultAllowedHostSuffixes are the tunnel domains accepted when
// APEXGURU_ALLOWED_HOSTS is unset. Loopback hosts are always accepted.
const defaultAllowedHostSuffixes = ".ngrok-free.app,.ngrok-free.dev,.ngrok.app,.ngrok.dev"

// newMux wires the authenticated /mcp endpoint and an open /healthz.
//
// The SDK's built-in DNS-rebinding guard rejects every non-loopback Host on
// loopback connections, which is exactly how ngrok forwards traffic. It is
// replaced with an explicit Host allowlist (allowedHosts), checked before the
// API key.
func newMux(server *mcp.Server, apiKey string, allowedHosts []string) *http.ServeMux {
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{DisableLocalhostProtection: true})
	mux := http.NewServeMux()
	mux.Handle("/mcp", requireHost(allowedHosts, requireAPIKey(apiKey, http.MaxBytesHandler(handler, maxRequestBytes))))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprintln(w, "ok"); err != nil {
			log.Printf("apexguru: write healthz: %v", err)
		}
	})
	return mux
}

func main() {
	s, err := loadSeed()
	if err != nil {
		log.Fatalf("apexguru: %v", err)
	}
	apiKey := os.Getenv("APEXGURU_MCP_API_KEY")
	if apiKey == "" {
		log.Fatalf("apexguru: APEXGURU_MCP_API_KEY must be set (static API key for MCP auth)")
	}

	allowed := os.Getenv("APEXGURU_ALLOWED_HOSTS")
	if allowed == "" {
		allowed = defaultAllowedHostSuffixes
	}
	allowedHosts := splitList(allowed)

	addr := ":" + port()
	srv := &http.Server{
		Addr:              addr,
		Handler:           newMux(newServer(s), apiKey, allowedHosts),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("apexguru: simulated ApexGuru MCP endpoint on %s/mcp (API key required)", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("apexguru: server error: %v", err)
	}
}

// requireAPIKey serves only requests that present the static API key, either
// as "Authorization: Bearer <key>" or "X-API-Key: <key>". The comparison is
// constant-time.
func requireAPIKey(want string, next http.Handler) http.Handler {
	wantBytes := []byte(want)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-API-Key")
		if got == "" {
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				got = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
			}
		}
		if subtle.ConstantTimeCompare([]byte(got), wantBytes) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="apexguru"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireHost rejects requests whose Host header is neither loopback nor
// matched by allowed. An entry beginning with "." matches any subdomain;
// other entries must match the host exactly. Ports are ignored.
func requireHost(allowed []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hostAllowed(r.Host, allowed) {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// hostAllowed reports whether host (optionally with port) passes the allowlist.
func hostAllowed(host string, allowed []string) bool {
	h := strings.ToLower(host)
	if hp, _, err := net.SplitHostPort(h); err == nil {
		h = hp
	}
	h = strings.Trim(h, "[]")
	if h == "localhost" || h == "127.0.0.1" || h == "::1" {
		return true
	}
	for _, a := range allowed {
		a = strings.ToLower(a)
		if strings.HasPrefix(a, ".") && strings.HasSuffix(h, a) && len(h) > len(a) {
			return true
		}
		if h == a {
			return true
		}
	}
	return false
}

// splitList splits a comma-separated env value, dropping blanks.
func splitList(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// port returns the listen port from $PORT, defaulting to 8766.
func port() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8766"
}
