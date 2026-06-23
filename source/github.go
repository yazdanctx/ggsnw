package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	maxPerPage = 100
	maxPages   = 10
	maxSleep   = 60 * time.Second
)

type searchResponse struct {
	TotalCount int `json:"total_count"`
	Items      []struct {
		Name string `json:"name"`
		Path string `json:"path"`
	} `json:"items"`
}

type GitHub struct {
	http  *http.Client
	token string
}

func NewGitHub(token string) *GitHub {
	return &GitHub{
		http:  &http.Client{Timeout: 15 * time.Second},
		token: token,
	}
}

func (g *GitHub) Name() string { return "github" }

func (g *GitHub) Expand(ctx context.Context, shortname string) ([]string, error) {
	var segments []string

	for page := 1; page <= maxPages; page++ {
		u := url.URL{
			Scheme: "https",
			Host:   "api.github.com",
			Path:   "/search/code",
		}
		q := u.Query()
		q.Set("q", "filename:"+shortname)
		q.Set("per_page", strconv.Itoa(maxPerPage))
		q.Set("page", strconv.Itoa(page))
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return segments, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+g.token)
		req.Header.Set("User-Agent", "ggsnw")

		resp, err := g.http.Do(req)
		if err != nil {
			return segments, err
		}

		remaining := resp.Header.Get("X-RateLimit-Remaining")
		reset := resp.Header.Get("X-RateLimit-Reset")

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return segments, err
		}

		if resp.StatusCode == 401 {
			return segments, fmt.Errorf("invalid GitHub token")
		}
		if resp.StatusCode == 403 && remaining == "0" {
			var resetUnix int64
			if reset != "" {
				fmt.Sscanf(reset, "%d", &resetUnix)
			}
			waitTime := time.Until(time.Unix(resetUnix, 0)) + time.Second
			if waitTime > 0 {
				waitTime = min(waitTime, maxSleep)
				select {
				case <-time.After(waitTime):
				case <-ctx.Done():
					return segments, ctx.Err()
				}
			}
			page--
			continue
		}
		if resp.StatusCode != 200 {
			return segments, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
		}

		var result searchResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return segments, err
		}

		if len(result.Items) == 0 {
			break
		}

		for _, item := range result.Items {
			parts := strings.Split(item.Path, "/")
			for _, part := range parts {
				if strings.Contains(strings.ToLower(part), strings.ToLower(shortname)) {
					segments = append(segments, part)
				}
			}
		}

		if page*maxPerPage >= result.TotalCount {
			break
		}

		if remaining != "" {
			rem := 0
			fmt.Sscanf(remaining, "%d", &rem)
			if rem <= 1 {
				var resetUnix int64
				if reset != "" {
					fmt.Sscanf(reset, "%d", &resetUnix)
				}
				waitTime := time.Until(time.Unix(resetUnix, 0)) + time.Second
				if waitTime > 0 {
					waitTime = min(waitTime, maxSleep)
					select {
					case <-time.After(waitTime):
					case <-ctx.Done():
						return segments, ctx.Err()
					}
				}
			} else if rem < 10 {
				select {
				case <-time.After(2 * time.Second):
				case <-ctx.Done():
					return segments, ctx.Err()
				}
			}
		}
	}

	return segments, nil
}
