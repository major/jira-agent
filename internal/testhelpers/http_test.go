package testhelpers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSONResponse(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	WriteJSONResponse(t, recorder, `{"ok":true}`)

	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}
	if got := recorder.Body.String(); got != `{"ok":true}` {
		t.Errorf("body = %q, want %q", got, `{"ok":true}`)
	}
}

func TestNewJSONServer(t *testing.T) {
	t.Parallel()

	server := NewJSONServer(t, http.MethodPost, "/rest/api/3/issue", `{"id":"10000"}`)

	request, err := http.NewRequest(http.MethodPost, server.URL+"/rest/api/3/issue", strings.NewReader(`{"fields":{}}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("post test server: %v", err)
	}
	defer response.Body.Close()

	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}
}

func TestDecodeJSONBody(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Sprint 1"}`))
	body := DecodeJSONBody(t, request)

	if got := body["name"]; got != "Sprint 1" {
		t.Errorf("name = %v, want %v", got, "Sprint 1")
	}
}
