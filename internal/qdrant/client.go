package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

const (
	vectorDims       = 1536    // text-embedding-3-small dimensions
	indexChunkSize   = 600     // characters per chunk
	indexChunkOverlap = 50     // overlap between consecutive chunks
	indexMaxFileSize = 100_000 // skip files larger than 100 KB
	indexMaxChunks   = 30      // max chunks per file
	indexBatchSize   = 20      // chunks per embedding API call
)

// Client provides higher-level operations (embed & search) against Qdrant.
type Client struct {
	baseAPIURL string
	llmClient  *openai.Client
	httpClient *http.Client
}

func NewClient(baseAPIURL string, llmClient *openai.Client) *Client {
	return &Client{
		baseAPIURL: baseAPIURL,
		llmClient:  llmClient,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// EnsureCollection creates the project's Qdrant collection if it doesn't exist.
func (c *Client) EnsureCollection(ctx context.Context, projectID int) error {
	collectionName := fmt.Sprintf("project_%d", projectID)

	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/collections/%s", c.baseAPIURL, collectionName), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode == 200 {
		return nil // already exists
	}

	payload := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     vectorDims,
			"distance": "Cosine",
		},
	}
	body, _ := json.Marshal(payload)
	req, err = http.NewRequestWithContext(ctx, "PUT",
		fmt.Sprintf("%s/collections/%s", c.baseAPIURL, collectionName),
		bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant: create collection %q: HTTP %d: %s", collectionName, resp.StatusCode, b)
	}
	return nil
}

// pointCount returns the number of indexed vectors in the project's collection.
func (c *Client) pointCount(ctx context.Context, projectID int) int {
	collectionName := fmt.Sprintf("project_%d", projectID)
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/collections/%s", c.baseAPIURL, collectionName), nil)
	if err != nil {
		return 0
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var result struct {
		Result struct {
			PointsCount int `json:"points_count"`
		} `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Result.PointsCount
}

// IndexDirectory indexes all text files in dirPath into the project's Qdrant collection.
// If the collection already has points it is a no-op (re-open is cheap).
func (c *Client) IndexDirectory(ctx context.Context, projectID int, dirPath string) error {
	if err := c.EnsureCollection(ctx, projectID); err != nil {
		return fmt.Errorf("ensure collection: %w", err)
	}

	if c.pointCount(ctx, projectID) > 0 {
		return nil // already indexed
	}

	collectionName := fmt.Sprintf("project_%d", projectID)

	type fileChunk struct {
		relPath  string
		content  string
		chunkIdx int
	}
	var chunks []fileChunk

	_ = filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "node_modules" || name == "vendor" ||
				name == "__pycache__" || name == "dist" || name == ".next" ||
				strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !isTextFile(name) {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > indexMaxFileSize {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		relPath, _ := filepath.Rel(dirPath, path)
		for i, ch := range chunkText(string(data)) {
			if i >= indexMaxChunks {
				break
			}
			chunks = append(chunks, fileChunk{relPath: relPath, content: ch, chunkIdx: i})
		}
		return nil
	})

	if len(chunks) == 0 {
		return nil
	}

	// Embed and upsert in batches
	pointID := 1
	for i := 0; i < len(chunks); i += indexBatchSize {
		end := min(i+indexBatchSize, len(chunks))
		batch := chunks[i:end]

		texts := make([]string, len(batch))
		for j, ch := range batch {
			texts[j] = ch.content
		}

		embedCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		resp, err := c.llmClient.CreateEmbeddings(embedCtx, openai.EmbeddingRequest{
			Input: texts,
			Model: openai.SmallEmbedding3,
		})
		cancel()
		if err != nil {
			continue // skip batch on error
		}

		type point struct {
			ID      int                    `json:"id"`
			Vector  []float32              `json:"vector"`
			Payload map[string]interface{} `json:"payload"`
		}
		var points []point
		for j, emb := range resp.Data {
			if j >= len(batch) {
				break
			}
			points = append(points, point{
				ID:     pointID,
				Vector: emb.Embedding,
				Payload: map[string]interface{}{
					"file_path": batch[j].relPath,
					"content":   batch[j].content,
					"chunk_idx": batch[j].chunkIdx,
				},
			})
			pointID++
		}

		upsertBody, _ := json.Marshal(map[string]interface{}{"points": points})
		req, _ := http.NewRequestWithContext(ctx, "PUT",
			fmt.Sprintf("%s/collections/%s/points", c.baseAPIURL, collectionName),
			bytes.NewBuffer(upsertBody))
		req.Header.Set("Content-Type", "application/json")
		upsertResp, err := c.httpClient.Do(req)
		if err != nil {
			continue
		}
		upsertResp.Body.Close()
	}

	return nil
}

// SearchContext retrieves context from the project's vector store based on the prompt.
func (c *Client) SearchContext(ctx context.Context, projectID int, query string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.llmClient.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{query},
		Model: openai.SmallEmbedding3,
	})
	if err != nil {
		return "", nil // embedding unavailable, skip context
	}
	if len(resp.Data) == 0 {
		return "", nil
	}
	vector := resp.Data[0].Embedding

	collectionName := fmt.Sprintf("project_%d", projectID)

	payload := map[string]interface{}{
		"vector":       vector,
		"limit":        5,
		"with_payload": true,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/collections/%s/points/search", c.baseAPIURL, collectionName),
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	qdrantResp, err := c.httpClient.Do(req)
	if err != nil {
		return "", nil
	}
	defer qdrantResp.Body.Close()

	if qdrantResp.StatusCode == 404 {
		return "", nil // collection not yet indexed
	}
	if qdrantResp.StatusCode != 200 {
		return "", nil
	}

	var result struct {
		Result []struct {
			Score   float64                `json:"score"`
			Payload map[string]interface{} `json:"payload"`
		} `json:"result"`
	}
	if err := json.NewDecoder(qdrantResp.Body).Decode(&result); err != nil {
		return "", nil
	}

	var sb strings.Builder
	for _, hit := range result.Result {
		if hit.Score < 0.3 {
			continue
		}
		if hit.Payload == nil {
			continue
		}
		fp, _ := hit.Payload["file_path"].(string)
		content, _ := hit.Payload["content"].(string)
		if fp != "" && content != "" {
			fmt.Fprintf(&sb, "File: %s\n```\n%s\n```\n\n", fp, content)
		} else if content != "" {
			fmt.Fprintf(&sb, "Context: %s\n\n", content)
		}
	}

	return sb.String(), nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

var textExtensions = map[string]bool{
	".go": true, ".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
	".java": true, ".c": true, ".cpp": true, ".h": true, ".hpp": true, ".cs": true,
	".rb": true, ".rs": true, ".swift": true, ".kt": true, ".scala": true,
	".sh": true, ".bash": true, ".zsh": true, ".fish": true,
	".md": true, ".txt": true, ".rst": true,
	".json": true, ".yaml": true, ".yml": true, ".toml": true, ".ini": true, ".env": true,
	".html": true, ".css": true, ".scss": true, ".sass": true,
	".sql": true, ".graphql": true, ".proto": true,
	".Makefile": true, ".dockerfile": true,
}

func isTextFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if textExtensions[ext] {
		return true
	}
	// Also match extensionless common files
	base := strings.ToLower(name)
	return base == "makefile" || base == "dockerfile" || base == "readme" || base == ".env"
}

func chunkText(text string) []string {
	if len(text) <= indexChunkSize {
		return []string{strings.TrimSpace(text)}
	}
	var chunks []string
	step := indexChunkSize - indexChunkOverlap
	for i := 0; i < len(text); i += step {
		end := min(i+indexChunkSize, len(text))
		chunk := strings.TrimSpace(text[i:end])
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(text) {
			break
		}
	}
	return chunks
}
