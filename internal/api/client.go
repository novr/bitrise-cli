package api

import (
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

const baseURL = "https://api.bitrise.io/v0.1"

type Client struct {
	token      string
	httpClient *http.Client
	backoff    time.Duration
}

func NewClient(token string) *Client {
	return &Client{
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		backoff:    500 * time.Millisecond,
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

type Build struct {
	Slug              string     `json:"slug"`
	BuildNumber       int        `json:"build_number"`
	Branch            string     `json:"branch"`
	CommitMessage     string     `json:"commit_message"`
	CommitHash        string     `json:"commit_hash"`
	TriggeredWorkflow string     `json:"triggered_workflow"`
	Status            int        `json:"status"`
	StatusText        string     `json:"status_text"`
	TriggeredAt       time.Time  `json:"triggered_at"`
	FinishedAt        *time.Time `json:"finished_at"`
	Duration          int        `json:"duration_in_seconds"`
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

func (c *Client) do(method, path string, params url.Values) ([]byte, error) {
	u := baseURL + path
	if params != nil {
		u += "?" + params.Encode()
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.backoff)
		}

		req, err := http.NewRequest(method, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", c.token)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Network errors are transient; retry.
			lastErr = err
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}

		// Retry on rate limiting and server errors, honoring Retry-After.
		if (resp.StatusCode == 429 || resp.StatusCode >= 500) && attempt < maxRetries {
			if d := retryAfter(resp); d > 0 {
				time.Sleep(d)
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

func (c *Client) GetMe() (*User, error) {
	body, err := c.do("GET", "/me", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data User `json:"data"`
	}
	return &resp.Data, json.Unmarshal(body, &resp)
}

func (c *Client) ListApps() ([]App, error) {
	body, err := c.do("GET", "/me/apps", url.Values{
		"limit":   {"100"},
		"sort_by": {"last_build_at"},
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []App `json:"data"`
	}
	return resp.Data, json.Unmarshal(body, &resp)
}

type ListBuildsParams struct {
	Limit       int
	Branch      string
	Workflow    string
	Status      string
	BuildNumber int
}

// maxPerPage is the largest page size the Bitrise API accepts per request.
const maxPerPage = 50

func (c *Client) ListBuilds(appSlug string, p ListBuildsParams) ([]Build, error) {
	baseQuery := url.Values{}
	if p.Branch != "" {
		baseQuery.Set("branch", p.Branch)
	}
	if p.Workflow != "" {
		baseQuery.Set("workflow", p.Workflow)
	}
	if p.Status != "" {
		baseQuery.Set("status", p.Status)
	}
	if p.BuildNumber > 0 {
		baseQuery.Set("build_number", strconv.Itoa(p.BuildNumber))
	}

	var all []Build
	next := ""
	for {
		q := url.Values{}
		for k, v := range baseQuery {
			q[k] = v
		}
		// Request only as many as still needed, capped at the API page size.
		pageSize := maxPerPage
		if p.Limit > 0 {
			remaining := p.Limit - len(all)
			if remaining < pageSize {
				pageSize = remaining
			}
		}
		q.Set("limit", strconv.Itoa(pageSize))
		if next != "" {
			q.Set("next", next)
		}

		body, err := c.do("GET", "/apps/"+appSlug+"/builds", q)
		if err != nil {
			return nil, err
		}
		var resp struct {
			Data   []Build `json:"data"`
			Paging struct {
				Next string `json:"next"`
			} `json:"paging"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Data...)

		if p.Limit > 0 && len(all) >= p.Limit {
			all = all[:p.Limit]
			break
		}
		if resp.Paging.Next == "" || len(resp.Data) == 0 {
			break
		}
		next = resp.Paging.Next
	}
	return all, nil
}

func (c *Client) GetBuildByNumber(appSlug string, buildNumber int) (*Build, error) {
	builds, err := c.ListBuilds(appSlug, ListBuildsParams{BuildNumber: buildNumber, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(builds) == 0 {
		return nil, fmt.Errorf("build #%d not found", buildNumber)
	}
	return &builds[0], nil
}

func (c *Client) GetBuildLog(buildSlug string) (*BuildLog, error) {
	body, err := c.do("GET", "/builds/"+buildSlug+"/log", nil)
	if err != nil {
		return nil, err
	}
	var logResp BuildLog
	return &logResp, json.Unmarshal(body, &logResp)
}

func (c *Client) DownloadRawLog(rawURL string) (string, error) {
	resp, err := c.httpClient.Get(rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FetchLog returns the full log text for a build, handling both archived and in-progress builds.
func (c *Client) FetchLog(buildSlug string) (string, bool, error) {
	logResp, err := c.GetBuildLog(buildSlug)
	if err != nil {
		return "", false, err
	}

	if logResp.IsArchived && logResp.ExpiringRawLogURL != "" {
		text, err := c.DownloadRawLog(logResp.ExpiringRawLogURL)
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
