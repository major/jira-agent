package commands

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/major/jira-agent/internal/testhelpers"
)

func TestChangelogBulkFetchCursorPagination(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/changelog/bulkfetch" {
			t.Errorf("path = %q, want /changelog/bulkfetch", r.URL.Path)
		}
		testhelpers.WriteJSONResponse(t, w, `{
			"issueChangeLogs": [
				{"issueId":"1","changeHistories":[]},
				{"issueId":"2","changeHistories":[]},
				{"issueId":"3","changeHistories":[]}
			],
			"nextPageToken": "page2tok",
			"isLast": false
		}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		issueChangelogCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"bulk-fetch", "--issues", "TEST-1,TEST-2,TEST-3", "--max-results", "100",
	)

	env := decodeEnvelope(t, buf.Bytes())
	pagination := requirePaginationMetadata(t, &env)
	if pagination.Type != "cursor" {
		t.Errorf("pagination type = %q, want cursor", pagination.Type)
	}
	if !pagination.HasMore {
		t.Error("pagination has_more = false, want true")
	}
	if pagination.Returned != 3 {
		t.Errorf("pagination returned = %d, want 3", pagination.Returned)
	}
	if pagination.NextToken != "page2tok" {
		t.Errorf("pagination next_token = %q, want page2tok", pagination.NextToken)
	}
	wantNextCommand := "changelog bulk-fetch --issues TEST-1,TEST-2,TEST-3 --max-results 100 --next-page-token page2tok"
	if pagination.NextCommand != wantNextCommand {
		t.Errorf("pagination next_command = %q, want %q", pagination.NextCommand, wantNextCommand)
	}
}

func TestChangelogBulkFetchEmptyResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testhelpers.WriteJSONResponse(t, w, `{
			"issueChangeLogs": [],
			"nextPageToken": "",
			"isLast": true
		}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		issueChangelogCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"bulk-fetch", "--issues", "TEST-99",
	)

	env := decodeEnvelope(t, buf.Bytes())
	pagination := requirePaginationMetadata(t, &env)
	if pagination.Type != "cursor" {
		t.Errorf("pagination type = %q, want cursor", pagination.Type)
	}
	if pagination.HasMore {
		t.Error("pagination has_more = true, want false")
	}
	if pagination.Returned != 0 {
		t.Errorf("pagination returned = %d, want 0", pagination.Returned)
	}
	if pagination.NextCommand != "" {
		t.Errorf("pagination next_command = %q, want empty", pagination.NextCommand)
	}
}
