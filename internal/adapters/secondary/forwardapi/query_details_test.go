package forwardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// queryLibraryServer serves a fwd repository of n queries. Each detail
// request takes delay and returns an intent; org has no queries.
func queryLibraryServer(t *testing.T, n int, delay time.Duration, inFlight, maxInFlight *int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/nqe/repos/org/commits/head/queries":
			_ = json.NewEncoder(w).Encode(NQEOrgQueriesResponse{})
		case r.URL.Path == "/api/nqe/repos/fwd/commits/head/queries":
			resp := NQEOrgQueriesResponse{}
			for i := 0; i < n; i++ {
				resp.Queries = append(resp.Queries, NQEOrgQuerySummary{
					Path:         fmt.Sprintf("/Q/%d", i),
					QueryID:      fmt.Sprintf("FQ_%d", i),
					LastCommitId: fmt.Sprintf("commit-%d", i),
				})
			}
			_ = json.NewEncoder(w).Encode(resp)
		case strings.HasPrefix(r.URL.Path, "/api/nqe/repos/fwd/commits/"):
			cur := atomic.AddInt64(inFlight, 1)
			for {
				prev := atomic.LoadInt64(maxInFlight)
				if cur <= prev || atomic.CompareAndSwapInt64(maxInFlight, prev, cur) {
					break
				}
			}
			defer atomic.AddInt64(inFlight, -1)
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
			path := r.URL.Query().Get("path")
			fmt.Fprintf(w, `{"intent":%q,"lastCommit":{"id":"c","committedAt":"2026-10-12T19:34:27.576021722Z"}}`, "Intent of "+path)
		default:
			http.NotFound(w, r)
		}
	}))
}

// TestGetNQEAllQueriesEnhanced_FetchesConcurrently checks that every query
// comes back with its intent, path and commit, fetched in parallel.
func TestGetNQEAllQueriesEnhanced_FetchesConcurrently(t *testing.T) {
	var inFlight, maxInFlight int64
	server := queryLibraryServer(t, 40, 20*time.Millisecond, &inFlight, &maxInFlight)
	defer server.Close()

	start := time.Now()
	queries, err := testClient(server.URL).GetNQEAllQueriesEnhanced(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetNQEAllQueriesEnhanced: %v", err)
	}
	elapsed := time.Since(start)

	if len(queries) != 40 {
		t.Fatalf("got %d queries, want 40", len(queries))
	}
	for _, q := range queries {
		if q.Intent != "Intent of "+q.Path || q.LastCommit.ID == "" || q.QueryID == "" {
			t.Errorf("query %+v is missing metadata", q)
			break
		}
	}
	if maxInFlight < 2 || maxInFlight > queryDetailWorkers {
		t.Errorf("max concurrent detail requests = %d, want 2..%d", maxInFlight, queryDetailWorkers)
	}
	// 40 requests of 20ms one at a time take 800ms.
	if elapsed > 600*time.Millisecond {
		t.Errorf("took %v; detail requests do not seem to run concurrently", elapsed)
	}
}

// TestGetNQEAllQueriesEnhanced_KeepsPartialResults checks that a load cut
// short by its context returns what it fetched instead of nothing.
func TestGetNQEAllQueriesEnhanced_KeepsPartialResults(t *testing.T) {
	var inFlight, maxInFlight int64
	server := queryLibraryServer(t, 200, 25*time.Millisecond, &inFlight, &maxInFlight)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	queries, err := testClient(server.URL).GetNQEAllQueriesEnhanced(ctx, nil)
	if err != nil {
		t.Fatalf("GetNQEAllQueriesEnhanced: %v", err)
	}
	if len(queries) == 0 || len(queries) >= 200 {
		t.Fatalf("got %d queries, want a partial result between 1 and 199", len(queries))
	}
	for _, q := range queries {
		if q.LastCommit.ID == "" {
			t.Fatalf("partial query %s has no commit ID, so it would be fetched again", q.Path)
		}
	}
}

// TestGetNQEQueryByCommit_DecodesLiveShape decodes a detail as the live API
// sends it: committedAt is an RFC 3339 string, not Unix seconds.
func TestGetNQEQueryByCommit_DecodesLiveShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"queryId":"FQ_34f8","commitCount":11,`+
			`"intent":"Verifies that enabled BGP peerings are established",`+
			`"lastCommit":{"id":"807dd695","authorEmail":"a@example.com","committedAt":"2026-10-12T19:34:27.576021722Z","title":"Commit","body":""},`+
			`"firstCommit":{"id":"bd37a283","committedAt":1606240650}}`)
	}))
	defer server.Close()

	d, err := testClient(server.URL).GetNQEQueryByCommit(context.Background(), "807dd695", "/L3/BGP/BGP Peerings Not Established", "fwd")
	if err != nil {
		t.Fatalf("GetNQEQueryByCommit: %v", err)
	}
	if d.Intent != "Verifies that enabled BGP peerings are established" {
		t.Errorf("intent = %q", d.Intent)
	}
	if want := time.Date(2026, 10, 12, 19, 34, 27, 0, time.UTC).Unix(); d.LastCommit.ID != "807dd695" || d.LastCommit.CommittedAt != want {
		t.Errorf("last commit = %+v, want id 807dd695 committed at %d", d.LastCommit, want)
	}
	if d.FirstCommit.CommittedAt != 1606240650 {
		t.Errorf("first commit time = %d, want 1606240650 (numeric form)", d.FirstCommit.CommittedAt)
	}
}
