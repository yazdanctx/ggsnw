package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	configDirName  = ".config/ggsnw"
	configFileName = "config.json"
	maxPerPage     = 100
	maxPages       = 10
)

type config struct {
	Token string `json:"token"`
}

type searchResponse struct {
	TotalCount int `json:"total_count"`
	Items      []struct {
		Name string `json:"name"`
		Path string `json:"path"`
	} `json:"items"`
}

type client struct {
	http  *http.Client
	token string
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, configDirName, configFileName)
}

func loadConfig() (*config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return nil, err
	}
	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveToken(token string) error {
	data, err := json.MarshalIndent(config{Token: token}, "", "  ")
	if err != nil {
		return err
	}
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func newClient(token string) *client {
	return &client{
		http:  &http.Client{Timeout: 15 * time.Second},
		token: token,
	}
}

func (c *client) searchShortname(query string) ([]string, error) {
	var segments []string

	for page := 1; page <= maxPages; page++ {
		u := url.URL{
			Scheme: "https",
			Host:   "api.github.com",
			Path:   "/search/code",
		}
		q := u.Query()
		q.Set("q", "filename:"+query)
		q.Set("per_page", strconv.Itoa(maxPerPage))
		q.Set("page", strconv.Itoa(page))
		u.RawQuery = q.Encode()

		req, err := http.NewRequest("GET", u.String(), nil)
		if err != nil {
			return segments, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("User-Agent", "ggsnw")

		resp, err := c.http.Do(req)
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
				fmt.Fprintf(os.Stderr, "Rate limited, waiting %v...\n", waitTime)
				time.Sleep(waitTime)
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
				if strings.Contains(strings.ToLower(part), strings.ToLower(query)) {
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
					fmt.Fprintf(os.Stderr, "Rate limit low, waiting %v...\n", waitTime)
					time.Sleep(waitTime)
				}
			} else if rem < 10 {
				time.Sleep(2 * time.Second)
			}
		}
	}

	return segments, nil
}

func main() {
	tokenPtr := flag.String("token", "", "GitHub personal access token")
	filePtr := flag.String("f", "", "file with shortnames (one per line)")
	outputPtr := flag.String("o", "", "output wordlist file")
	flag.Parse()

	token := *tokenPtr
	if token == "" {
		if cfg, err := loadConfig(); err == nil {
			token = cfg.Token
		}
	}
	if *tokenPtr != "" {
		if err := saveToken(token); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save token: %v\n", err)
		}
	}
	if token == "" {
		fmt.Println("ggsnw needs a GitHub token to search code (30 req/min vs 10 unauthenticated).")
		fmt.Println("Get one: https://github.com/settings/tokens (no scopes needed for public repos)")
		fmt.Print("Enter token: ")
		fmt.Scan(&token)
		token = strings.TrimSpace(token)
		if token == "" {
			fmt.Fprintln(os.Stderr, "Token required.")
			os.Exit(1)
		}
		if err := saveToken(token); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save token: %v\n", err)
		}
	}

	var queries []string
	if *filePtr != "" {
		data, err := os.ReadFile(*filePtr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				queries = append(queries, line)
			}
		}
	} else if flag.NArg() > 0 {
		queries = append(queries, flag.Arg(0))
	} else {
		fmt.Fprintln(os.Stderr, "Usage: ggsnw [--token TOKEN] [-f file] [-o file] [SHORTNAME]")
		os.Exit(1)
	}

	c := newClient(token)
	var allWords []string
	seen := make(map[string]bool)

	for _, q := range queries {
		words, err := c.searchShortname(q)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error searching %q: %v\n", q, err)
			continue
		}
		if len(words) == 0 {
			fmt.Printf("\t[ ] %s not found\n", q)
		}
		for _, w := range words {
			if !seen[w] {
				seen[w] = true
				allWords = append(allWords, w)
				fmt.Printf("\t[x] %s\n", w)
			}
		}
	}

	sort.Strings(allWords)
	fmt.Println("-----")
	for _, w := range allWords {
		fmt.Println(w)
	}
	fmt.Println("-----")

	if *outputPtr != "" {
		data := strings.Join(allWords, "\n") + "\n"
		if err := os.WriteFile(*outputPtr, []byte(data), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
		}
	}
}
