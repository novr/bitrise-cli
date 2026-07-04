package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestClient returns a client pointed at srv with retries sped up.
func newTestClient(baseURL string) *Client {
	c := NewClient("test-token")
	c.baseURL = baseURL
	c.backoff = 0
	return c
}

func TestListBuildsPagination(t *testing.T) {
	var gotNext []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next := r.URL.Query().Get("next")
		gotNext = append(gotNext, next)

		builds := make([]map[string]interface{}, 50)
		body := map[string]interface{}{"data": builds}
		if next == "" {
			// first page points to the second
			body["paging"] = map[string]string{"next": "tok2"}
		}
		json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	builds, err := c.ListBuilds(context.Background(), "app-slug", ListBuildsParams{Limit: 60})
	if err != nil {
		t.Fatalf("ListBuilds: %v", err)
	}
	if len(builds) != 60 {
		t.Errorf("got %d builds, want 60 (capped at limit across pages)", len(builds))
	}
	if len(gotNext) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(gotNext))
	}
	if gotNext[0] != "" || gotNext[1] != "tok2" {
		t.Errorf("next tokens = %v, want [\"\", \"tok2\"]", gotNext)
	}
}

func TestListBuildsStopsWhenNoNext(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// 10 builds, no paging.next -> should stop after one page
		builds := make([]map[string]interface{}, 10)
		json.NewEncoder(w).Encode(map[string]interface{}{"data": builds})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	builds, err := c.ListBuilds(context.Background(), "app-slug", ListBuildsParams{Limit: 100})
	if err != nil {
		t.Fatalf("ListBuilds: %v", err)
	}
	if len(builds) != 10 {
		t.Errorf("got %d builds, want 10", len(builds))
	}
	if calls != 1 {
		t.Errorf("made %d requests, want 1", calls)
	}
}

func TestListAppsPagination(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		body := map[string]interface{}{"data": make([]map[string]interface{}, 50)}
		if page == 1 {
			body["paging"] = map[string]string{"next": "p2"}
		}
		json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	apps, err := c.ListApps(context.Background())
	if err != nil {
		t.Fatalf("ListApps: %v", err)
	}
	if len(apps) != 100 {
		t.Errorf("got %d apps across pages, want 100", len(apps))
	}
}

func TestRetryOn503(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]string{"username": "alice"},
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	user, err := c.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe after retries: %v", err)
	}
	if user.Username != "alice" {
		t.Errorf("username = %q, want alice", user.Username)
	}
	if calls != 3 {
		t.Errorf("handler hit %d times, want 3", calls)
	}
}

func TestContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable) // would otherwise be retried
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.backoff = time.Hour // if cancellation is ignored, the retry sleep would hang
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() { _, err := c.GetMe(ctx); done <- err }()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("call did not return promptly after cancellation")
	}
}

func TestRateLimitedExhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.GetMe(context.Background())
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("error = %q, want rate-limited message", err)
	}
}

func TestAuthAndErrorMapping(t *testing.T) {
	t.Run("401", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		c := newTestClient(srv.URL)
		if _, err := c.GetMe(context.Background()); err == nil || !strings.Contains(err.Error(), "authentication failed") {
			t.Errorf("err = %v, want authentication failed", err)
		}
	})

	t.Run("message body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"message": "app not found"})
		}))
		defer srv.Close()
		c := newTestClient(srv.URL)
		if _, err := c.GetMe(context.Background()); err == nil || !strings.Contains(err.Error(), "app not found") {
			t.Errorf("err = %v, want surfaced API message", err)
		}
	})
}

func TestFetchLogChunkOrdering(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"is_archived": false,
			"log_chunks": []map[string]interface{}{
				{"chunk": "third", "position": 2},
				{"chunk": "first", "position": 0},
				{"chunk": "second", "position": 1},
			},
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	text, archived, err := c.FetchLog(context.Background(), "app-slug", "build-slug")
	if err != nil {
		t.Fatalf("FetchLog: %v", err)
	}
	if archived {
		t.Error("archived = true, want false for chunked log")
	}
	if text != "firstsecondthird" {
		t.Errorf("log = %q, want %q (position order)", text, "firstsecondthird")
	}
}

func TestDownloadRawLogExpired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "<html>Access Denied</html>")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.DownloadRawLog(context.Background(), srv.URL); err == nil {
		t.Error("expected error for 403 (expired URL), got nil")
	}
}

func TestFetchLogArchived(t *testing.T) {
	var rawSrv *httptest.Server
	rawSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "RAW ARCHIVED LOG")
	}))
	defer rawSrv.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"is_archived":          true,
			"expiring_raw_log_url": rawSrv.URL,
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	text, archived, err := c.FetchLog(context.Background(), "app-slug", "build-slug")
	if err != nil {
		t.Fatalf("FetchLog: %v", err)
	}
	if !archived {
		t.Error("archived = false, want true")
	}
	if text != "RAW ARCHIVED LOG" {
		t.Errorf("log = %q, want raw archived body", text)
	}
}
