package usecases

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/forward-mcp/internal/domain"
)

// TestRunNQEQueryByIDCachedPageKeepsTruncationHint checks that a page served
// from the semantic cache reads like a fresh one: same header, same hint that
// more rows may exist.
func TestRunNQEQueryByIDCachedPageKeepsTruncationHint(t *testing.T) {
	service := createTestService()
	queryID := "FQ_ac651cb2901b067fe7dbfb511613ab44776d8029"
	items := make([]map[string]interface{}, 30)
	for i := range items {
		items[i] = map[string]interface{}{"device": fmt.Sprintf("device-%d", i)}
	}
	mock := service.forwardClient.(*MockForwardClient)
	mock.nqeResult = &domain.NQERunResult{SnapshotID: "snapshot-123", Items: items}

	args := RunNQEQueryByIDArgs{QueryID: queryID, NetworkID: "162112", Options: &NQEQueryOptions{Limit: 5}}
	fresh, err := service.RunNQEQueryByID(context.Background(), args)
	if err != nil {
		t.Fatalf("fresh run: %v", err)
	}
	mock.lastByIDParams = nil
	cached, err := service.RunNQEQueryByID(context.Background(), args)
	if err != nil {
		t.Fatalf("cached run: %v", err)
	}
	if mock.lastByIDParams != nil {
		t.Fatal("second run called the API; expected a cache hit")
	}

	for name, text := range map[string]string{"fresh": fresh.Text, "cached": cached.Text} {
		if !strings.Contains(text, "NQE query completed. Found 5 items") {
			t.Errorf("%s response has no row count header:\n%s", name, text)
		}
		if !strings.Contains(text, "Results may be truncated") {
			t.Errorf("%s response has no truncation hint:\n%s", name, text)
		}
	}
	if fresh.Text != cached.Text {
		t.Errorf("cached response differs from fresh one:\nfresh:\n%s\ncached:\n%s", fresh.Text, cached.Text)
	}
}

// TestRunNQEQueryByIDInvalidIDError checks that a malformed query ID gets an
// error that says what a query ID looks like and where to find one.
func TestRunNQEQueryByIDInvalidIDError(t *testing.T) {
	service := createTestService()
	mock := service.forwardClient.(*MockForwardClient)
	mock.shouldError = true
	mock.errorMessage = "bad request (HTTP 400): A SHA-1 digest is 40 hex digits, not 12"

	_, err := service.RunNQEQueryByID(context.Background(), RunNQEQueryByIDArgs{QueryID: "FQ_bogus", NetworkID: "162112"})
	if err == nil {
		t.Fatal("expected an error for a malformed query ID")
	}
	for _, want := range []string{`"FQ_bogus"`, "40 hex digits", "search_nqe_queries"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
	if strings.Contains(err.Error(), "SHA-1") {
		t.Errorf("error %q still passes through the raw API message", err)
	}
}

// TestSearchPathsSingleQueryWording checks that search_paths does not call
// itself a bulk search.
func TestSearchPathsSingleQueryWording(t *testing.T) {
	service := createTestService()

	result, err := service.SearchPathsEntry(context.Background(), SearchPathsArgs{NetworkID: "162112", SrcIP: "10.0.0.1", DstIP: "10.0.0.2"})
	if err != nil {
		t.Fatalf("search_paths: %v", err)
	}
	if !strings.HasPrefix(result.Text, "Path search completed.") {
		t.Errorf("response should start with %q:\n%s", "Path search completed.", result.Text)
	}
	if strings.Contains(result.Text, "Bulk") {
		t.Errorf("single path search response mentions bulk:\n%s", result.Text)
	}
}
