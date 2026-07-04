package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.bitrise.io/v0.1"

// Version is the br CLI version, surfaced via `br version` and the User-Agent.
const Version = "0.1.0"

// userAgent identifies br in Bitrise's traffic logs (helps support triage).
const userAgent = "br-cli/" + Version

type Client struct {
	token      string
	httpClient *http.Client
	backoff    time.Duration
	baseURL    string
}

func NewClient(token string) *Client {
	return &Client{
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		backoff:    500 * time.Millisecond,
		baseURL:    defaultBaseURL,
	}
}

type User struct {
	Slug     string `json:"slug"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type App struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	RepoURL string `json:"repo_url"`
}

// BuildStatus is the Bitrise build status code (values fixed by the API).
type BuildStatus int

const (
	StatusRunning BuildStatus = 0 // not finished
	StatusSuccess BuildStatus = 1
	StatusFailed  BuildStatus = 2
	StatusError   BuildStatus = 3
	StatusAborted BuildStatus = 4
)

type Build struct {
	Slug              string      `json:"slug"`
	BuildNumber       int         `json:"build_number"`
	Branch            string      `json:"branch"`
	CommitMessage     string      `json:"commit_message"`
	CommitHash        string      `json:"commit_hash"`
	TriggeredWorkflow string      `json:"triggered_workflow"`
	Status            BuildStatus `json:"status"`
	StatusText        string      `json:"status_text"`
	TriggeredAt       time.Time   `json:"triggered_at"`
	FinishedAt        *time.Time  `json:"finished_at"`
	Duration          int         `json:"duration_in_seconds"`
}

type BuildLog struct {
	ExpiringRawLogURL string `json:"expiring_raw_log_url"`
	IsArchived        bool   `json:"is_archived"`
	LogChunks         []struct {
		Chunk    string `json:"chunk"`
		Position int    `json:"position"`
	} `json:"log_chunks"`
}

const maxRetries = 3

func (c *Client) do(ctx context.Context, method, path string, params url.Values) ([]byte, error) {
	u := c.baseURL + path
	if params != nil {
		u += "?" + params.Encode()
	}

	var lastErr error
	var wait time.Duration // single pacing point; 0 on the first attempt
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if wait > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		wait = c.backoff

		req, err := http.NewRequestWithContext(ctx, method, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", c.token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", userAgent)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err() // cancelled; do not retry
			}
			lastErr = err // network errors are transient; retry
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}

		// Retry rate limiting and server errors; Retry-After is a lower bound.
		if (resp.StatusCode == 429 || resp.StatusCode >= 500) && attempt < maxRetries {
			if d := retryAfter(resp); d > wait {
				wait = d
			}
			lastErr = fmt.Errorf("API error %d", resp.StatusCode)
			continue
		}

		if resp.StatusCode == 401 {
			return nil, fmt.Errorf("authentication failed: run 'br auth login'")
		}
		if resp.StatusCode == 429 {
			return nil, fmt.Errorf("rate limited by Bitrise API; try again shortly")
		}
		if resp.StatusCode >= 400 {
			var errResp struct {
				Message string `json:"message"`
			}
			if json.Unmarshal(body, &errResp) == nil && errResp.Message != "" {
				return nil, fmt.Errorf("API error: %s", errResp.Message)
			}
			return nil, fmt.Errorf("API error %d", resp.StatusCode)
		}
		return body, nil
	}
	return nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
}

// retryAfter parses the Retry-After header, supporting the delay-seconds form.
func retryAfter(resp *http.Response) time.Duration {
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}

func (c *Client) GetMe(ctx context.Context) (*User, error) {
	body, err := c.do(ctx, "GET", "/me", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data User `json:"data"`
	}
	return &resp.Data, json.Unmarshal(body, &resp)
}

func (c *Client) ListApps(ctx context.Context) ([]App, error) {
	return fetchPaged[App](ctx, c, "/me/apps", url.Values{"sort_by": {"last_build_at"}}, 0)
}

type ListBuildsParams struct {
	Limit       int
	Branch      string
	Workflow    string
	Status      *BuildStatus // nil means no status filter
	BuildNumber int
}

// maxPerPage is the largest page size the Bitrise API accepts per request.
const maxPerPage = 50

// fetchPaged GETs path following paging.next until exhausted. If limit > 0 it
// stops once limit items are collected (capping the final page); limit <= 0
// fetches every page.
func fetchPaged[T any](ctx context.Context, c *Client, path string, base url.Values, limit int) ([]T, error) {
	var all []T
	next := ""
	for {
		q := url.Values{}
		for k, v := range base {
			q[k] = v
		}
		pageSize := maxPerPage
		if limit > 0 {
			if remaining := limit - len(all); remaining < pageSize {
				pageSize = remaining
			}
		}
		q.Set("limit", strconv.Itoa(pageSize))
		if next != "" {
			q.Set("next", next)
		}

		body, err := c.do(ctx, "GET", path, q)
		if err != nil {
			return nil, err
		}
		var resp struct {
			Data   []T `json:"data"`
			Paging struct {
				Next string `json:"next"`
			} `json:"paging"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Data...)

		if limit > 0 && len(all) >= limit {
			return all[:limit], nil
		}
		if resp.Paging.Next == "" || len(resp.Data) == 0 {
			return all, nil
		}
		next = resp.Paging.Next
	}
}

func (c *Client) ListBuilds(ctx context.Context, appSlug string, p ListBuildsParams) ([]Build, error) {
	q := url.Values{}
	if p.Branch != "" {
		q.Set("branch", p.Branch)
	}
	if p.Workflow != "" {
		q.Set("workflow", p.Workflow)
	}
	if p.Status != nil {
		q.Set("status", strconv.Itoa(int(*p.Status)))
	}
	if p.BuildNumber > 0 {
		q.Set("build_number", strconv.Itoa(p.BuildNumber))
	}
	return fetchPaged[Build](ctx, c, "/apps/"+appSlug+"/builds", q, p.Limit)
}

func (c *Client) GetBuildByNumber(ctx context.Context, appSlug string, buildNumber int) (*Build, error) {
	builds, err := c.ListBuilds(ctx, appSlug, ListBuildsParams{BuildNumber: buildNumber, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(builds) == 0 {
		return nil, fmt.Errorf("build #%d not found", buildNumber)
	}
	return &builds[0], nil
}

func (c *Client) GetBuildLog(ctx context.Context, buildSlug string) (*BuildLog, error) {
	body, err := c.do(ctx, "GET", "/builds/"+buildSlug+"/log", nil)
	if err != nil {
		return nil, err
	}
	var logResp BuildLog
	return &logResp, json.Unmarshal(body, &logResp)
}

func (c *Client) DownloadRawLog(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	// The signed URL is short-lived; a non-2xx here is typically an expired link
	// returning an HTML error page, which must not be mistaken for log content.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("failed to download log (status %d); the log URL may have expired", resp.StatusCode)
	}
	return string(data), nil
}

// FetchLog returns the full log text for a build, handling both archived and in-progress builds.
func (c *Client) FetchLog(ctx context.Context, buildSlug string) (string, bool, error) {
	logResp, err := c.GetBuildLog(ctx, buildSlug)
	if err != nil {
		return "", false, err
	}

	if logResp.IsArchived && logResp.ExpiringRawLogURL != "" {
		text, err := c.DownloadRawLog(ctx, logResp.ExpiringRawLogURL)
		return text, true, err
	}

	// In-progress build: concatenate available chunks sorted by position
	chunks := logResp.LogChunks
	if len(chunks) == 0 {
		return "", false, nil
	}
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Position < chunks[j].Position
	})
	var sb strings.Builder
	for _, ch := range chunks {
		sb.WriteString(ch.Chunk)
	}
	return sb.String(), false, nil
}
