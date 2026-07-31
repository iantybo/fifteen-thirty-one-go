package service

import (
	"errors"
	"io"
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

// errClose is the sentinel returned by failingCloser.Close.
var errClose = errors.New("close failed")

// failingCloser reads normally but always fails to close.
type failingCloser struct{ io.Reader }

func (failingCloser) Close() error { return errClose }

// newRequestFailingClose builds a request whose body fails on Close.
func newRequestFailingClose(body string) *http.Request {
	r := newRequest(body)
	r.Body = failingCloser{Reader: strings.NewReader(body)}
	return r
}

// TestDecodeJSONValidBody verifies a well-formed body decodes into v.
func TestDecodeJSONValidBody(t *testing.T) {
	var got payload
	if err := DecodeJSON(newRequest(`{"name":"yankees"}`), &got); err != nil {
		t.Fatalf("DecodeJSON() = %v, want nil", err)
	}
	if got.Name != "yankees" {
		t.Errorf("Name = %q, want %q", got.Name, "yankees")
	}
}

// TestDecodeJSONMalformedBody verifies truncated JSON is rejected.
func TestDecodeJSONMalformedBody(t *testing.T) {
	var got payload
	if err := DecodeJSON(newRequest(`{"name":`), &got); err == nil {
		t.Fatal("DecodeJSON() = nil, want error for truncated JSON")
	}
}

// TestDecodeJSONTrailingData is the regression test for the limit bypass: a
// complete JSON value must not decode cleanly when extra data follows it. The
// suffix is deliberately small so this covers trailing data alone; body size is
// TestDecodeJSONOversizedBody's job.
func TestDecodeJSONTrailingData(t *testing.T) {
	body := `{"name":"yankees"}{"name":"redsox"}`

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

// TestDecodeJSONAtLimit verifies a body of exactly maxRequestBody bytes is
// still accepted, guarding against an off-by-one in the size check.
func TestDecodeJSONAtLimit(t *testing.T) {
	const prefix, suffix = `{"name":"`, `"}`
	body := prefix + strings.Repeat("x", maxRequestBody-len(prefix)-len(suffix)) + suffix

	if len(body) != maxRequestBody {
		t.Fatalf("test setup: body length = %d, want %d", len(body), maxRequestBody)
	}

	var got payload
	if err := DecodeJSON(newRequest(body), &got); err != nil {
		t.Fatalf("DecodeJSON() = %v, want nil for body at maxRequestBody limit", err)
	}
}

// TestDecodeJSONCloseErrorWithValidBody checks a close failure is surfaced even
// when the body itself decoded successfully.
func TestDecodeJSONCloseErrorWithValidBody(t *testing.T) {
	var got payload
	err := DecodeJSON(newRequestFailingClose(`{"name":"yankees"}`), &got)
	if !errors.Is(err, errClose) {
		t.Fatalf("DecodeJSON() = %v, want error wrapping errClose", err)
	}
	if got.Name != "yankees" {
		t.Errorf("Name = %q, want %q; body should still decode", got.Name, "yankees")
	}
}

// TestDecodeJSONCloseErrorWithMalformedBody checks that when both the decode
// and the close fail, neither error is lost.
func TestDecodeJSONCloseErrorWithMalformedBody(t *testing.T) {
	var got payload
	err := DecodeJSON(newRequestFailingClose(`{"name":`), &got)
	if !errors.Is(err, errClose) {
		t.Fatalf("DecodeJSON() = %v, want error wrapping errClose", err)
	}
	if !strings.Contains(err.Error(), "decode request body") {
		t.Errorf("DecodeJSON() = %v, want it to also report the decode failure", err)
	}
}
