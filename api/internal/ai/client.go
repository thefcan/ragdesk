// Package ai is an HTTP client for the Python AI service.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client calls the ragdesk AI service.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
	stream  *http.Client
}

// NewClient returns a Client targeting the AI service base URL. token is sent as
// a shared internal secret so the AI service can reject foreign callers.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{Timeout: 120 * time.Second},
		stream:  &http.Client{}, // no timeout; the request context governs streaming
	}
}

func (c *Client) newRequest(ctx context.Context, path string, payload any) (*http.Request, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("X-Internal-Token", c.token)
	}
	return req, nil
}

type ingestRequest struct {
	DocumentID  string `json:"document_id"`
	WorkspaceID string `json:"workspace_id"`
	Text        string `json:"text"`
}

type ingestResponse struct {
	ChunkCount int `json:"chunk_count"`
}

// Ingest asks the AI service to chunk, embed and store a document.
func (c *Client) Ingest(ctx context.Context, documentID, workspaceID, text string) (int, error) {
	req, err := c.newRequest(ctx, "/ingest", ingestRequest{DocumentID: documentID, WorkspaceID: workspaceID, Text: text})
	if err != nil {
		return 0, err
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

type chatRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Question    string `json:"question"`
}

// Chat streams a RAG answer from the AI service straight through to w (NDJSON).
// On an upstream failure it emits an error event so the client always receives
// a well-formed stream.
func (c *Client) Chat(ctx context.Context, workspaceID, question string, w http.ResponseWriter) error {
	flusher, _ := w.(http.Flusher)
	emitError := func() {
		_, _ = io.WriteString(w, `{"type":"error","content":"the assistant is unavailable"}`+"\n")
		if flusher != nil {
			flusher.Flush()
		}
	}

	req, err := c.newRequest(ctx, "/chat", chatRequest{WorkspaceID: workspaceID, Question: question})
	if err != nil {
		emitError()
		return err
	}
	resp, err := c.stream.Do(req)
	if err != nil {
		emitError()
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		emitError()
		return fmt.Errorf("ai chat: unexpected status %d", resp.StatusCode)
	}

	buf := make([]byte, 4096)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if errors.Is(rerr, io.EOF) {
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}
