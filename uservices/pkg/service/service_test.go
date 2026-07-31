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

// errRead is the sentinel returned by failingReader.Read.
var errRead = errors.New("read failed")

// failingReader always fails to read, and closes cleanly.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errRead }

func (failingReader) Close() error { return nil }

// newRequestFailingRead builds a request whose body fails on Read.
func newRequestFailingRead() *http.Request {
	r := newRequest("")
	r.Body = failingReader{}
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
	err := DecodeJSON(newRequest(`{"name":`), &got)
	if err == nil {
		t.Fatal("DecodeJSON() = nil, want error for truncated JSON")
	}
	if !errors.Is(err, ErrDecodeBody) {
		t.Errorf("DecodeJSON() = %v, want error wrapping ErrDecodeBody", err)
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

// TestDecodeJSONOversizedAfterCompleteValue isolates size enforcement from
// value shape: the first maxRequestBody bytes are a complete, parseable value,
// and only the trailing byte pushes the body over the cap. Paired with
// TestDecodeJSONAtLimit this pins the boundary from both sides.
func TestDecodeJSONOversizedAfterCompleteValue(t *testing.T) {
	const value = `{"name":"yankees"}`
	body := strings.Repeat(" ", maxRequestBody-len(value)) + value + " "

	if len(body) != maxRequestBody+1 {
		t.Fatalf("test setup: body length = %d, want %d", len(body), maxRequestBody+1)
	}

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

// TestDecodeJSONReadError checks a transport read failure is propagated to the
// caller rather than being reported as malformed JSON.
func TestDecodeJSONReadError(t *testing.T) {
	var got payload
	if err := DecodeJSON(newRequestFailingRead(), &got); !errors.Is(err, errRead) {
		t.Fatalf("DecodeJSON() = %v, want error wrapping errRead", err)
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
	if !errors.Is(err, ErrDecodeBody) {
		t.Errorf("DecodeJSON() = %v, want it to also wrap ErrDecodeBody", err)
	}
}
