package testhelpers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// WriteJSONResponse writes a JSON response body with the content type expected by the Jira client.
func WriteJSONResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatalf("write response: %v", err)
	}
}

// NewJSONServer returns a test server that validates the request method and path before writing JSON.
func NewJSONServer(t *testing.T, method, path, response string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			t.Errorf("method = %q, want %q", r.Method, method)
		}
		if r.URL.Path != path {
			t.Errorf("path = %q, want %q", r.URL.Path, path)
		}
		WriteJSONResponse(t, w, response)
	}))
	t.Cleanup(server.Close)
	return server
}

// DecodeJSONBody decodes a JSON request body into a map for endpoint tests.
func DecodeJSONBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	body := map[string]any{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	return body
}

// NewServer returns a test server that validates the request method and path
// before calling the provided handler function to write the response.
func NewServer(t *testing.T, method, path string, handler func(http.ResponseWriter)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			t.Errorf("method = %q, want %q", r.Method, method)
		}
		if r.URL.Path != path {
			t.Errorf("path = %q, want %q", r.URL.Path, path)
		}
		handler(w)
	}))
	t.Cleanup(server.Close)
	return server
}
