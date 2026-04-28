// Package client provides an authenticated HTTP client for the Jira Cloud REST API.
//
// All requests include Basic authentication, JSON content headers, and
// automatic error mapping for non-2xx responses. The client supports both
// the standard REST API v3 (/rest/api/3/) and the Agile API (/rest/agile/1.0/).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	apperr "github.com/major/jira-agent/internal/errors"
)

const (
	// defaultTimeout is the overall request timeout for the Jira API client.
	// Covers the full request lifecycle: DNS, connect, TLS handshake, sending
	// the request, and reading the response. 30 seconds is generous for a REST
	// API but prevents indefinite hangs on network issues.
	defaultTimeout = 30 * time.Second

	// maxResponseSize caps how many bytes we'll read from any API response.
	// Prevents a misbehaving server from sending a huge payload that exhausts
	// memory. 10 MB covers the largest Jira responses (bulk search results
	// with many fields expanded).
	maxResponseSize = 10 * 1024 * 1024 // 10 MB

	// defaultUserAgent identifies this client to the Jira API. Overridden at
	// build time via WithUserAgent to include the real version from ldflags.
	defaultUserAgent = "jira-agent/dev"
)

// Ref holds a lazily-populated reference to a Client. Command constructors
// capture the Ref at build time; the Before hook populates it after
// authentication, so all commands share the live client.
type Ref struct {
	*Client
}

// Client is an authenticated HTTP client for the Jira Cloud REST API.
type Client struct {
	baseURL      string
	agileBaseURL string
	httpClient   *http.Client
	authHeader   string
	userAgent    string
	logger       *slog.Logger
}

// MultipartFile describes one file part in a multipart/form-data request.
type MultipartFile struct {
	FieldName string
	FileName  string
	Reader    io.Reader
}

// Option is a functional option for NewClient.
type Option func(*Client)

// NewClient creates a new Client with the given auth header and options.
// The authHeader should be the full "Basic ..." value from Config.BasicAuthHeader().
func NewClient(authHeader string, opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		authHeader: authHeader,
		userAgent:  defaultUserAgent,
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithBaseURL sets the base URL for the client.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithAgileBaseURL sets the base URL for Jira Software Agile API requests.
func WithAgileBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.agileBaseURL = baseURL
	}
}

// WithHTTPClient sets the underlying HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithLogger sets the structured logger for request debugging.
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) {
		c.logger = l
	}
}

// WithUserAgent sets the User-Agent header sent with every request.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		c.userAgent = ua
	}
}

// DoRequest is the core request method that handles authentication, serialization,
// and error mapping for all HTTP methods. It is exported so command packages
// can make arbitrary API calls when needed.
func (c *Client) DoRequest(ctx context.Context, method, path string, body, result any) error {
	return c.doRequest(ctx, method, c.baseURL, path, body, result)
}

func (c *Client) doRequest(ctx context.Context, method, baseURL, path string, body, result any) error {
	var reqBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(encoded)
	}

	endpoint := baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reqBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	// Only set Content-Type when sending a body. Some APIs return errors
	// on GET requests that include Content-Type: application/json.
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.logger.Debug("http request", "method", method, "path", path)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	return handleResponse(resp, result)
}

func handleResponse(resp *http.Response, result any) error {
	defer resp.Body.Close()

	// Read the full response body for error messages or JSON decoding.
	// Capped at maxResponseSize to prevent memory exhaustion.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	// Map non-2xx status codes to typed errors.
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return apperr.NewAuthError(
			fmt.Sprintf("authentication failed (HTTP %d)", resp.StatusCode),
			nil,
			apperr.WithDetails("check your JIRA_EMAIL and JIRA_API_KEY credentials"),
		)
	}
	if resp.StatusCode == http.StatusNotFound {
		return apperr.NewNotFoundError(
			fmt.Sprintf("resource not found (HTTP %d)", resp.StatusCode),
			nil,
		)
	}
	if resp.StatusCode >= 400 {
		return apperr.NewAPIError(
			fmt.Sprintf("API error (HTTP %d)", resp.StatusCode),
			resp.StatusCode,
			string(respBody),
			nil,
		)
	}

	// Decode JSON response if a result target was provided and there is a body.
	if result != nil && len(respBody) > 0 {
		// Validate Content-Type before attempting JSON decode. Without this,
		// an HTML error page from a proxy or maintenance window produces a
		// cryptic json.Unmarshal error instead of a clear diagnostic.
		ct := resp.Header.Get("Content-Type")
		if ct != "" {
			mediaType, _, parseErr := mime.ParseMediaType(ct)
			if parseErr == nil && mediaType != "application/json" {
				preview := string(respBody)
				if len(preview) > 200 {
					preview = preview[:200] + "..."
				}
				return fmt.Errorf("unexpected Content-Type %q (expected application/json): %s", ct, preview)
			}
		}

		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// Get performs a GET request with optional query parameters.
// Values are percent-encoded via url.Values to handle special characters safely.
func (c *Client) Get(ctx context.Context, path string, params map[string]string, result any) error {
	path = appendParams(path, params)
	return c.DoRequest(ctx, http.MethodGet, path, nil, result)
}

// AgileGet performs a GET request against the Jira Software Agile API.
func (c *Client) AgileGet(ctx context.Context, path string, params map[string]string, result any) error {
	path = appendParams(path, params)
	return c.doRequest(ctx, http.MethodGet, c.agileBaseURL, path, nil, result)
}

// AgilePost performs a POST request against the Jira Software Agile API.
func (c *Client) AgilePost(ctx context.Context, path string, body, result any) error {
	return c.doRequest(ctx, http.MethodPost, c.agileBaseURL, path, body, result)
}

// AgilePut performs a PUT request against the Jira Software Agile API.
func (c *Client) AgilePut(ctx context.Context, path string, body, result any) error {
	return c.doRequest(ctx, http.MethodPut, c.agileBaseURL, path, body, result)
}

// AgileDelete performs a DELETE request against the Jira Software Agile API.
func (c *Client) AgileDelete(ctx context.Context, path string, result any) error {
	return c.doRequest(ctx, http.MethodDelete, c.agileBaseURL, path, nil, result)
}

func appendParams(path string, params map[string]string) string {
	if len(params) > 0 {
		q := url.Values{}
		for k, v := range params {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	return path
}

// Post performs a POST request with JSON body.
func (c *Client) Post(ctx context.Context, path string, body, result any) error {
	return c.DoRequest(ctx, http.MethodPost, path, body, result)
}

// Put performs a PUT request with JSON body.
func (c *Client) Put(ctx context.Context, path string, body, result any) error {
	return c.DoRequest(ctx, http.MethodPut, path, body, result)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string, result any) error {
	return c.DoRequest(ctx, http.MethodDelete, path, nil, result)
}

// PostMultipart performs a multipart/form-data POST request.
// Jira attachment upload requires X-Atlassian-Token: no-check, so this helper
// sets it for all multipart calls made by the CLI.
func (c *Client) PostMultipart(ctx context.Context, path string, files []MultipartFile, result any) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, file := range files {
		part, err := writer.CreateFormFile(file.FieldName, file.FileName)
		if err != nil {
			return fmt.Errorf("create multipart file part: %w", err)
		}
		if _, err := io.Copy(part, file.Reader); err != nil {
			return fmt.Errorf("copy multipart file: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	endpoint := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Atlassian-Token", "no-check")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	return handleResponse(resp, result)
}
