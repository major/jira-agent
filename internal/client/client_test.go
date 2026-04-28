package client

import (
	"bytes"
	"context"
	stderrors "errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/testhelpers"
)

func TestNewClient_Options(t *testing.T) {
	t.Parallel()

	hc := &http.Client{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := NewClient(
		"Basic token",
		WithBaseURL("https://jira.example/rest/api/3"),
		WithAgileBaseURL("https://jira.example/rest/agile/1.0"),
		WithHTTPClient(hc),
		WithLogger(logger),
		WithUserAgent("jira-agent/test"),
	)

	if c.authHeader != "Basic token" {
		t.Errorf("authHeader = %q, want %q", c.authHeader, "Basic token")
	}
	if c.baseURL != "https://jira.example/rest/api/3" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "https://jira.example/rest/api/3")
	}
	if c.agileBaseURL != "https://jira.example/rest/agile/1.0" {
		t.Errorf("agileBaseURL = %q, want %q", c.agileBaseURL, "https://jira.example/rest/agile/1.0")
	}
	if c.httpClient != hc {
		t.Errorf("httpClient = %p, want %p", c.httpClient, hc)
	}
	if c.logger != logger {
		t.Errorf("logger = %p, want %p", c.logger, logger)
	}
	if c.userAgent != "jira-agent/test" {
		t.Errorf("userAgent = %q, want %q", c.userAgent, "jira-agent/test")
	}
}

func TestClient_AgileGet_UsesAgileBaseURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/agile/1.0/board" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/board")
		}
		if got := r.URL.Query().Get("type"); got != "scrum" {
			t.Errorf("type = %q, want %q", got, "scrum")
		}
		testhelpers.WriteJSONResponse(t, w, `{"values":[]}`)
	}))
	defer server.Close()

	c := NewClient(
		"Basic token",
		WithBaseURL(server.URL+"/rest/api/3"),
		WithAgileBaseURL(server.URL+"/rest/agile/1.0"),
	)
	var result map[string]any
	if err := c.AgileGet(context.Background(), "/board", map[string]string{"type": "scrum"}, &result); err != nil {
		t.Fatalf("AgileGet() error = %v, want nil", err)
	}
}

func TestClient_AgilePost_UsesAgileBaseURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/rest/agile/1.0/sprint" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/sprint")
		}
		body := testhelpers.DecodeJSONBody(t, r)
		if body["name"] != "Sprint 1" {
			t.Errorf("name = %v, want %v", body["name"], "Sprint 1")
		}
		testhelpers.WriteJSONResponse(t, w, `{"id":1}`)
	}))
	defer server.Close()

	c := NewClient(
		"Basic token",
		WithBaseURL(server.URL+"/rest/api/3"),
		WithAgileBaseURL(server.URL+"/rest/agile/1.0"),
	)
	var result map[string]any
	if err := c.AgilePost(context.Background(), "/sprint", map[string]any{"name": "Sprint 1"}, &result); err != nil {
		t.Fatalf("AgilePost() error = %v, want nil", err)
	}
	if result["id"] != float64(1) {
		t.Errorf("result[id] = %v, want %v", result["id"], float64(1))
	}
}

func TestClient_AgilePut_UsesAgileBaseURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
		}
		if r.URL.Path != "/rest/agile/1.0/sprint/1" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/sprint/1")
		}
		body := testhelpers.DecodeJSONBody(t, r)
		if body["name"] != "Sprint 1 Updated" {
			t.Errorf("name = %v, want %v", body["name"], "Sprint 1 Updated")
		}
		testhelpers.WriteJSONResponse(t, w, `{"ok":true}`)
	}))
	defer server.Close()

	c := NewClient(
		"Basic token",
		WithBaseURL(server.URL+"/rest/api/3"),
		WithAgileBaseURL(server.URL+"/rest/agile/1.0"),
	)
	var result map[string]any
	if err := c.AgilePut(context.Background(), "/sprint/1", map[string]any{"name": "Sprint 1 Updated"}, &result); err != nil {
		t.Fatalf("AgilePut() error = %v, want nil", err)
	}
	if result["ok"] != true {
		t.Errorf("result[ok] = %v, want %v", result["ok"], true)
	}
}

func TestClient_AgileDelete_UsesAgileBaseURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
		}
		if r.URL.Path != "/rest/agile/1.0/sprint/1" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/sprint/1")
		}
		if r.Header.Get("Content-Type") != "" {
			t.Errorf("Content-Type = %q, want empty", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := NewClient(
		"Basic token",
		WithBaseURL(server.URL+"/rest/api/3"),
		WithAgileBaseURL(server.URL+"/rest/agile/1.0"),
	)
	if err := c.AgileDelete(context.Background(), "/sprint/1", nil); err != nil {
		t.Fatalf("AgileDelete() error = %v, want nil", err)
	}
}

func TestClient_DoRequest_SendsHeadersAndJSONBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/issue" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/issue")
		}
		if got := r.Header.Get("Authorization"); got != "Basic token" {
			t.Errorf("Authorization = %q, want %q", got, "Basic token")
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want %q", got, "application/json")
		}
		if got := r.Header.Get("User-Agent"); got != "jira-agent/test" {
			t.Errorf("User-Agent = %q, want %q", got, "jira-agent/test")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want %q", got, "application/json")
		}

		body := testhelpers.DecodeJSONBody(t, r)
		if body["summary"] != "test" {
			t.Errorf("summary = %v, want %v", body["summary"], "test")
		}

		testhelpers.WriteJSONResponse(t, w, `{"ok":true}`)
	}))
	defer server.Close()

	c := NewClient("Basic token", WithBaseURL(server.URL), WithUserAgent("jira-agent/test"))
	var result map[string]any
	if err := c.Post(context.Background(), "/issue", map[string]any{"summary": "test"}, &result); err != nil {
		t.Fatalf("Post() error = %v, want nil", err)
	}
	if result["ok"] != true {
		t.Errorf("result[ok] = %v, want %v", result["ok"], true)
	}
}

func TestClient_Get_EncodesQueryAndOmitsContentType(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "" {
			t.Errorf("Content-Type = %q, want empty", got)
		}
		if got := r.URL.Query().Get("jql"); got != `project = "TEST"` {
			t.Errorf("jql = %q, want %q", got, `project = "TEST"`)
		}
		testhelpers.WriteJSONResponse(t, w, `{"issues":[]}`)
	}))
	defer server.Close()

	c := NewClient("Basic token", WithBaseURL(server.URL))
	var result map[string]any
	if err := c.Get(context.Background(), "/search", map[string]string{"jql": `project = "TEST"`}, &result); err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}
}

func TestClient_DoRequest_StatusErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		wantCode   int
		wantErrAs  any
	}{
		{name: "unauthorized", statusCode: http.StatusUnauthorized, wantCode: 3, wantErrAs: &apperr.AuthError{}},
		{name: "forbidden", statusCode: http.StatusForbidden, wantCode: 3, wantErrAs: &apperr.AuthError{}},
		{name: "not found", statusCode: http.StatusNotFound, wantCode: 2, wantErrAs: &apperr.NotFoundError{}},
		{name: "server error", statusCode: http.StatusInternalServerError, wantCode: 4, wantErrAs: &apperr.APIError{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(`{"error":"bad"}`))
			}))
			defer server.Close()

			c := NewClient("Basic token", WithBaseURL(server.URL))
			err := c.Get(context.Background(), "/issue/TEST-1", nil, nil)
			if err == nil {
				t.Fatal("Get() error = nil, want error")
			}
			if got := apperr.ExitCodeFor(err); got != tt.wantCode {
				t.Errorf("ExitCodeFor() = %d, want %d", got, tt.wantCode)
			}

			switch tt.wantErrAs.(type) {
			case *apperr.AuthError:
				var target *apperr.AuthError
				if !stderrors.As(err, &target) {
					t.Errorf("errors.As(AuthError) = false, want true")
				}
			case *apperr.NotFoundError:
				var target *apperr.NotFoundError
				if !stderrors.As(err, &target) {
					t.Errorf("errors.As(NotFoundError) = false, want true")
				}
			case *apperr.APIError:
				var target *apperr.APIError
				if !stderrors.As(err, &target) {
					t.Errorf("errors.As(APIError) = false, want true")
				}
			}
		})
	}
}

func TestClient_DoRequest_RejectsNonJSONResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
		body        string
		wantErr     string
	}{
		{
			name:        "text html with charset rejected",
			contentType: "text/html; charset=utf-8",
			body:        "<html>maintenance</html>",
			wantErr:     "unexpected Content-Type",
		},
		{
			name:        "text plain with charset rejected",
			contentType: "text/plain; charset=utf-8",
			body:        `{"ok":true}`,
			wantErr:     "unexpected Content-Type",
		},
		{
			name:        "application json with charset accepted",
			contentType: "application/json; charset=utf-8",
			body:        `{"ok":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.contentType != "" {
					w.Header().Set("Content-Type", tt.contentType)
				}
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			c := NewClient("Basic token", WithBaseURL(server.URL))
			var result map[string]any
			err := c.Get(context.Background(), "/myself", nil, &result)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("Get() error = nil, want error")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Get() error = %q, want %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Get() error = %v, want nil", err)
			}
			if result["ok"] != true {
				t.Errorf("result[ok] = %v, want %v", result["ok"], true)
			}
		})
	}
}

func TestClient_DoRequest_AcceptsMissingContentType(t *testing.T) {
	t.Parallel()

	c := NewClient(
		"Basic token",
		WithBaseURL("https://jira.example"),
		WithHTTPClient(&http.Client{Transport: staticRoundTripper{body: `{"ok":true}`}}),
	)
	var result map[string]any
	if err := c.Get(context.Background(), "/myself", nil, &result); err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}
	if result["ok"] != true {
		t.Errorf("result[ok] = %v, want %v", result["ok"], true)
	}
}

func TestClient_DoRequest_CapsAPIErrorResponseBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(bytes.Repeat([]byte("x"), maxResponseSize+1))
	}))
	defer server.Close()

	c := NewClient("Basic token", WithBaseURL(server.URL))
	err := c.Get(context.Background(), "/huge-error", nil, nil)
	if err == nil {
		t.Fatal("Get() error = nil, want error")
	}

	var apiErr *apperr.APIError
	if !stderrors.As(err, &apiErr) {
		t.Fatalf("errors.As(APIError) = false for %T", err)
	}
	if got := len(apiErr.Body); got != maxResponseSize {
		t.Errorf("len(APIError.Body) = %d, want %d", got, maxResponseSize)
	}
}

func TestClient_DoRequest_UnmarshalableBody(t *testing.T) {
	t.Parallel()

	c := NewClient("Basic token", WithBaseURL("https://jira.example"))
	err := c.Post(context.Background(), "/issue", map[string]any{"bad": make(chan int)}, nil)
	if err == nil {
		t.Fatal("Post() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "marshal request body") {
		t.Errorf("Post() error = %q, want marshal request body", err.Error())
	}
}

func TestClient_PostMultipart_RejectsNonJSONResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if got := r.Header.Get("X-Atlassian-Token"); got != "no-check" {
			t.Errorf("X-Atlassian-Token = %q, want %q", got, "no-check")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html>maintenance</html>"))
	}))
	defer server.Close()

	c := NewClient("Basic token", WithBaseURL(server.URL))
	files := []MultipartFile{{
		FieldName: "file",
		FileName:  "test.txt",
		Reader:    strings.NewReader("hello"),
	}}
	var result map[string]any
	err := c.PostMultipart(context.Background(), "/attachment", files, &result)
	if err == nil {
		t.Fatal("PostMultipart() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unexpected Content-Type") {
		t.Errorf("PostMultipart() error = %q, want unexpected Content-Type", err.Error())
	}
}

func TestClient_PostMultipart_UploadsFilesAndHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/attachment" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/attachment")
		}
		if got := r.Header.Get("Authorization"); got != "Basic token" {
			t.Errorf("Authorization = %q, want %q", got, "Basic token")
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want %q", got, "application/json")
		}
		if got := r.Header.Get("User-Agent"); got != "jira-agent/test" {
			t.Errorf("User-Agent = %q, want %q", got, "jira-agent/test")
		}
		if got := r.Header.Get("X-Atlassian-Token"); got != "no-check" {
			t.Errorf("X-Atlassian-Token = %q, want %q", got, "no-check")
		}
		if got := r.Header.Get("Content-Type"); !strings.HasPrefix(got, "multipart/form-data; boundary=") {
			t.Errorf("Content-Type = %q, want multipart/form-data with boundary", got)
		}

		if err := r.ParseMultipartForm(1024); err != nil {
			t.Fatalf("ParseMultipartForm() error = %v, want nil", err)
		}
		files := r.MultipartForm.File["file"]
		if got, want := len(files), 2; got != want {
			t.Fatalf("len(files) = %d, want %d", got, want)
		}
		assertMultipartFile(t, files[0], "first.txt", "hello")
		assertMultipartFile(t, files[1], "second.txt", "world")

		testhelpers.WriteJSONResponse(t, w, `{"ok":true}`)
	}))
	defer server.Close()

	c := NewClient(
		"Basic token",
		WithBaseURL(server.URL),
		WithUserAgent("jira-agent/test"),
	)
	files := []MultipartFile{
		{FieldName: "file", FileName: "first.txt", Reader: strings.NewReader("hello")},
		{FieldName: "file", FileName: "second.txt", Reader: strings.NewReader("world")},
	}
	var result map[string]any
	if err := c.PostMultipart(context.Background(), "/attachment", files, &result); err != nil {
		t.Fatalf("PostMultipart() error = %v, want nil", err)
	}
	if result["ok"] != true {
		t.Errorf("result[ok] = %v, want %v", result["ok"], true)
	}
}

func TestClient_PostMultipart_ReturnsReaderError(t *testing.T) {
	t.Parallel()

	readErr := stderrors.New("read failed")
	c := NewClient("Basic token", WithBaseURL("https://jira.example"))
	files := []MultipartFile{{
		FieldName: "file",
		FileName:  "broken.txt",
		Reader:    failingReader{err: readErr},
	}}
	err := c.PostMultipart(context.Background(), "/attachment", files, nil)
	if err == nil {
		t.Fatal("PostMultipart() error = nil, want error")
	}
	if !stderrors.Is(err, readErr) {
		t.Errorf("errors.Is(readErr) = false for %v", err)
	}
	if !strings.Contains(err.Error(), "copy multipart file") {
		t.Errorf("PostMultipart() error = %q, want copy multipart file", err.Error())
	}
}

func TestClient_PutAndDelete(t *testing.T) {
	t.Parallel()

	t.Run("put", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
			}
			if r.URL.Path != "/issue/TEST-1" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if body["summary"] != "updated" {
				t.Errorf("summary = %v, want %v", body["summary"], "updated")
			}
			testhelpers.WriteJSONResponse(t, w, `{"ok":true}`)
		}))
		defer server.Close()

		c := NewClient("Basic token", WithBaseURL(server.URL))
		var result map[string]any
		if err := c.Put(context.Background(), "/issue/TEST-1", map[string]any{"summary": "updated"}, &result); err != nil {
			t.Fatalf("Put() error = %v, want nil", err)
		}
		if result["ok"] != true {
			t.Errorf("result[ok] = %v, want %v", result["ok"], true)
		}
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.Header.Get("Content-Type") != "" {
				t.Errorf("Content-Type = %q, want empty", r.Header.Get("Content-Type"))
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		c := NewClient("Basic token", WithBaseURL(server.URL))
		if err := c.Delete(context.Background(), "/issue/TEST-1", nil); err != nil {
			t.Fatalf("Delete() error = %v, want nil", err)
		}
	})
}

type staticRoundTripper struct {
	body string
}

func (rt staticRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(rt.body)),
		Request:    req,
	}, nil
}

type failingReader struct {
	err error
}

func (r failingReader) Read([]byte) (int, error) {
	return 0, r.err
}

func assertMultipartFile(t *testing.T, file *multipart.FileHeader, wantName, wantBody string) {
	t.Helper()

	if file.Filename != wantName {
		t.Errorf("Filename = %q, want %q", file.Filename, wantName)
	}
	opened, err := file.Open()
	if err != nil {
		t.Fatalf("Open() error = %v, want nil", err)
	}
	defer opened.Close()

	body, err := io.ReadAll(opened)
	if err != nil {
		t.Fatalf("ReadAll() error = %v, want nil", err)
	}
	if got := string(body); got != wantBody {
		t.Errorf("body = %q, want %q", got, wantBody)
	}
}
