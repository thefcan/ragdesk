package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleHealth verifies the liveness probe responds 200 with the expected
// payload without touching any external dependency.
func TestHandleHealth(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	s.handleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Status != "ok" || got.Service != "ragdesk-api" {
		t.Fatalf("unexpected body: %+v", got)
	}
}
