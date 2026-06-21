package main

import (
	"bytes"
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
	maxSleep      = 60 * time.Second
)

type config struct {
	Token       string `json:"token"`
	GitHubToken string `json:"github_token"`
	OpenAIKey   string `json:"openai_key"`
}

func (c *config) githubToken() string {
	if c.GitHubToken != "" {
		return c.GitHubToken
	}
	return c.Token
}

type searchResponse struct {
	TotalCount int `json:"total_count"`
	Items      []struct {
		Name string `json:"name"`
		Path string `json:"path"`
	} `json:"items"`
}

type openAIRequest struct {
	Model    string           `json:"model"`
	Messages []openAIMessage  `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type client struct {
	http       *http.Client
	token      string
	openAIKey  string
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

func saveConfig(update func(*config)) error {
	cfg := &config{}
	if data, err := os.ReadFile(configPath()); err == nil {
		json.Unmarshal(data, cfg)
	}
	update(cfg)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func newClient(token, openAIKey string) *client {
	return &client{
		http:      &http.Client{Timeout: 15 * time.Second},
		token:     token,
		openAIKey: openAIKey,
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
				waitTime = min(waitTime, maxSleep)
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
					waitTime = min(waitTime, maxSleep)
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

func (c *client) aiGuess(query string, guesses int) ([]string, error) {
	prefix := strings.SplitN(query, "~", 2)[0]

	body := openAIRequest{
		Model: "gpt-3.5-turbo",
		Messages: []openAIMessage{
			{
				Role:    "system",
				Content: "Return only a list of words, separated by newlines, and nothing else. Ensure that the words contain only alphanumeric characters.",
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Make a list of guesses, for what the rest of the word could be from this snippet. Ensure that the snippet is a substring of your guess. Make %d guesses. Snippet: %s", guesses, prefix),
			},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.openAIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OpenAI API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result openAIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI returned no choices")
	}

	content := strings.TrimSpace(result.Choices[0].Message.Content)
	var words []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			words = append(words, line)
		}
	}

	return words, nil
}

func main() {
	tokenPtr := flag.String("token", "", "GitHub personal access token")
	keyPtr := flag.String("key", "", "OpenAI API key")
	modePtr := flag.String("mode", "github", "search mode: github or ai")
	guessesPtr := flag.Int("guesses", 10, "number of AI guesses per shortname")
	filePtr := flag.String("f", "", "file with shortnames (one per line)")
	outputPtr := flag.String("o", "", "output wordlist file")
	flag.Parse()

	mode := *modePtr
	if mode != "github" && mode != "ai" {
		fmt.Fprintf(os.Stderr, "Invalid mode %q. Use \"github\" or \"ai\".\n", mode)
		os.Exit(1)
	}
	if *guessesPtr < 1 {
		fmt.Fprintln(os.Stderr, "Guesses must be at least 1.")
		os.Exit(1)
	}

	var token string
	var openAIKey string

	if mode == "github" {
		token = *tokenPtr
		if token == "" {
			if cfg, err := loadConfig(); err == nil {
				token = cfg.githubToken()
			}
		}
		if *tokenPtr != "" {
			saveConfig(func(cfg *config) { cfg.GitHubToken = token })
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
			saveConfig(func(cfg *config) { cfg.GitHubToken = token })
		}
	} else {
		openAIKey = *keyPtr
		if openAIKey == "" {
			if cfg, err := loadConfig(); err == nil {
				openAIKey = cfg.OpenAIKey
			}
		}
		if *keyPtr != "" {
			saveConfig(func(cfg *config) { cfg.OpenAIKey = openAIKey })
		}
		if openAIKey == "" {
			fmt.Println("ggsnw needs an OpenAI API key for AI mode.")
			fmt.Println("Get one: https://platform.openai.com/api-keys")
			fmt.Print("Enter key: ")
			fmt.Scan(&openAIKey)
			openAIKey = strings.TrimSpace(openAIKey)
			if openAIKey == "" {
				fmt.Fprintln(os.Stderr, "Key required.")
				os.Exit(1)
			}
			saveConfig(func(cfg *config) { cfg.OpenAIKey = openAIKey })
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
		fmt.Fprintf(os.Stderr, "Usage: ggsnw [--token TOKEN] [--key KEY] [--mode github|ai] [--guesses N] [-f file] [-o file] [SHORTNAME]\n")
		os.Exit(1)
	}

	c := newClient(token, openAIKey)
	var allWords []string
	seen := make(map[string]bool)

	for _, q := range queries {
		var words []string
		var err error

		if mode == "github" {
			words, err = c.searchShortname(q)
		} else {
			words, err = c.aiGuess(q, *guessesPtr)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing %q: %v\n", q, err)
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
