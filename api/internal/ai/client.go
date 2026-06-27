// Package ai is an HTTP client for the Python AI service.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client calls the ragdesk AI service.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewClient returns a Client targeting the AI service base URL. token is sent as
// a shared internal secret so the AI service can reject foreign callers.
func NewClient(baseURL, token string) *Client {
	return &Client{baseURL: baseURL, token: token, http: &http.Client{Timeout: 120 * time.Second}}
}

type ingestRequest struct {
	DocumentID  string `json:"document_id"`
	WorkspaceID string `json:"workspace_id"`
	Text        string `json:"text"`
}

type ingestResponse struct {
	ChunkCount int `json:"chunk_count"`
}

// Ingest asks the AI service to chunk, embed and store a document, returning
// the number of chunks written.
func (c *Client) Ingest(ctx context.Context, documentID, workspaceID, text string) (int, error) {
	body, err := json.Marshal(ingestRequest{DocumentID: documentID, WorkspaceID: workspaceID, Text: text})
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/ingest", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("X-Internal-Token", c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("ai ingest: unexpected status %d", resp.StatusCode)
	}
	var out ingestResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, err
	}
	return out.ChunkCount, nil
}
