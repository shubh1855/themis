package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sashabaranov/go-openai"
)

// Client provides higher-level operations (embed & search) against Qdrant.
type Client struct {
	baseAPIURL  string
	llmClient   *openai.Client
	httpClient  *http.Client
}

func NewClient(baseAPIURL string, llmClient *openai.Client) *Client {
	return &Client{
		baseAPIURL: baseAPIURL,
		llmClient:  llmClient,
		httpClient: &http.Client{},
	}
}

// SearchContext retrieves context from the project's vector store based on the prompt.
func (c *Client) SearchContext(ctx context.Context, projectID int, query string) (string, error) {
	// Fast timeout so simple prompts like "hi" don't hang if the embedder API is slow
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// 1. Get embedding for the query
	resp, err := c.llmClient.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{query},
		Model: openai.SmallEmbedding3, // Assuming this maps to a valid embedding model on the proxy
	})
	if err != nil {
		return "", fmt.Errorf("embedding failed: %w", err)
	}
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("no embedding returned")
	}
	vector := resp.Data[0].Embedding

	collectionName := fmt.Sprintf("project_%d", projectID)

	// 2. Query Qdrant
	payload := map[string]interface{}{
		"vector": vector,
		"limit":  5,
		"with_payload": true,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/collections/%s/points/search", c.baseAPIURL, collectionName), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	qdrantResp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("qdrant search failed: %w", err)
	}
	defer qdrantResp.Body.Close()

	if qdrantResp.StatusCode == 404 {
		// Collection doesn't exist yet, meaning no memory has been indexed.
		return "", nil
	}

	if qdrantResp.StatusCode != 200 {
		b, _ := io.ReadAll(qdrantResp.Body)
		return "", fmt.Errorf("qdrant returned %d: %s", qdrantResp.StatusCode, b)
	}

	var result struct {
		Result []struct {
			Score   float64                `json:"score"`
			Payload map[string]interface{} `json:"payload"`
		} `json:"result"`
	}
	if err := json.NewDecoder(qdrantResp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode qdrant response: %w", err)
	}

	if len(result.Result) == 0 {
		return "", nil
	}

	var contextStr string
	for _, hit := range result.Result {
		// Minimum relevance threshold
		if hit.Score < 0.3 {
			continue
		}
		
		var filepath, content string
		if hit.Payload != nil {
			if fp, ok := hit.Payload["file_path"].(string); ok {
				filepath = fp
			}
			if txt, ok := hit.Payload["content"].(string); ok {
				content = txt
			}
		}
		if filepath != "" && content != "" {
			contextStr += fmt.Sprintf("File: %s\n```\n%s\n```\n\n", filepath, content)
		} else if content != "" {
			contextStr += fmt.Sprintf("Context: %s\n\n", content)
		}
	}

	return contextStr, nil
}
