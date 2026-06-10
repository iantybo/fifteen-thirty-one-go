// Package meshclient is the shared HTTP client every microservice uses to talk
// to its peers. Service addresses are resolved by name through the catalog, so
// callers refer to peers by their catalog name (e.g. "yankees") rather than
// hard-coding hosts.
package meshclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"fifteen-thirty-one-go/uservices/pkg/catalog"
)

// defaultTimeout bounds every peer call so a slow downstream cannot wedge a
// request handler indefinitely.
const defaultTimeout = 5 * time.Second

// Client calls peer services by their catalog name.
type Client struct {
	http    *http.Client
	baseURL func(name string) string
}

// New returns a Client whose peer addresses are resolved via resolveBaseURL.
// If resolveBaseURL is nil, addresses fall back to the MESH_<NAME>_URL
// environment variable, then to http://<name>.svc.cluster.local:8080.
func New(resolveBaseURL func(name string) string) *Client {
	if resolveBaseURL == nil {
		resolveBaseURL = defaultResolve
	}
	return &Client{
		http:    &http.Client{Timeout: defaultTimeout},
		baseURL: resolveBaseURL,
	}
}

// defaultResolve maps a catalog name to a base URL, preferring an explicit
// environment override so the mesh can run locally on distinct ports.
func defaultResolve(name string) string {
	if v := os.Getenv("MESH_" + envKey(name) + "_URL"); v != "" {
		return v
	}
	return fmt.Sprintf("http://%s.svc.cluster.local:8080", name)
}

// envKey upper-cases a service name for use in an environment variable.
func envKey(name string) string {
	out := make([]byte, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}

// Call POSTs req (JSON-encoded) to peer's /score endpoint and decodes the
// JSON response into out. peer must be a known catalog service.
func (c *Client) Call(ctx context.Context, peer, path string, req, out any) error {
	if _, ok := catalog.Lookup(peer); !ok {
		return fmt.Errorf("meshclient: unknown peer %q", peer)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("meshclient: encode request for %q: %w", peer, err)
	}

	url := c.baseURL(peer) + path
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("meshclient: build request for %q: %w", peer, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("meshclient: call %q: %w", peer, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("meshclient: peer %q returned status %d", peer, resp.StatusCode)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("meshclient: decode response from %q: %w", peer, err)
		}
	}
	return nil
}

// Fanout is a single peer call to issue as part of a concurrent fan-out: POST
// Req to Peer's Path and decode the JSON response into the value Out points to.
type Fanout struct {
	Peer string
	Path string
	Req  any
	Out  any
}

// CallAll issues every call in fanouts concurrently against the shared context
// and waits for all of them to finish. Each call decodes into its own Out, so
// callers can merge the results in a deterministic order afterwards. It returns
// the joined error of every failed call (preserving fan-out order), or nil if
// every call succeeded; ctx is shared, so one failure does not cancel the
// others — the per-call timeout still bounds them. Callers that only need to
// know whether the fan-out failed can compare the result against nil as usual,
// while those that want every peer's failure can inspect it with errors.Is/As.
func (c *Client) CallAll(ctx context.Context, fanouts ...Fanout) error {
	errs := make([]error, len(fanouts))
	var wg sync.WaitGroup
	wg.Add(len(fanouts))
	for i, f := range fanouts {
		go func() {
			defer wg.Done()
			errs[i] = c.Call(ctx, f.Peer, f.Path, f.Req, f.Out)
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}
