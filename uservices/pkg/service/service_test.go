package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type payload struct {
	Name string `json:"name"`
}

// newRequest builds a POST request whose body is the given string.
func newRequest(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
}

func TestDecodeJSONValidBody(t *testing.T) {
	var got payload
	if err := DecodeJSON(newRequest(`{"name":"yankees"}`), &got); err != nil {
		t.Fatalf("DecodeJSON() = %v, want nil", err)
	}
	if got.Name != "yankees" {
		t.Errorf("Name = %q, want %q", got.Name, "yankees")
	}
}

func TestDecodeJSONMalformedBody(t *testing.T) {
	var got payload
	if err := DecodeJSON(newRequest(`{"name":`), &got); err == nil {
		t.Fatal("DecodeJSON() = nil, want error for truncated JSON")
	}
}

// TestDecodeJSONTrailingData is the regression test for the limit bypass: a
// complete JSON value followed by more than maxRequestBody bytes must be
// rejected rather than decoded as if the trailing data were not there.
func TestDecodeJSONTrailingData(t *testing.T) {
	body := `{"name":"yankees"}` + strings.Repeat("x", maxRequestBody+1)

	var got payload
	if err := DecodeJSON(newRequest(body), &got); err == nil {
		t.Fatal("DecodeJSON() = nil, want error for data trailing the JSON value")
	}
}

// TestDecodeJSONOversizedBody covers a single JSON value that on its own
// exceeds the cap; it must be rejected, not silently truncated into something
// that happens to parse.
func TestDecodeJSONOversizedBody(t *testing.T) {
	body := `{"name":"` + strings.Repeat("x", maxRequestBody+1) + `"}`

	var got payload
	if err := DecodeJSON(newRequest(body), &got); err == nil {
		t.Fatal("DecodeJSON() = nil, want error for body exceeding maxRequestBody")
	}
}
