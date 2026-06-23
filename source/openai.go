package source

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
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

type OpenAI struct {
	http      *http.Client
	apiKey    string
	Guesses   int
}

func NewOpenAI(apiKey string) *OpenAI {
	return &OpenAI{
		http:    &http.Client{Timeout: 30 * time.Second},
		apiKey:  apiKey,
		Guesses: 10,
	}
}

func (o *OpenAI) Name() string { return "ai" }

func (o *OpenAI) Expand(ctx context.Context, shortname string) ([]string, error) {
	prefix := strings.SplitN(shortname, "~", 2)[0]

	guesses := o.Guesses
	if guesses < 1 {
		guesses = 10
	}

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

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.http.Do(req)
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
