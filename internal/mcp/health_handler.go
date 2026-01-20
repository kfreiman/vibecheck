package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HealthResponse represents the JSON response for health endpoints
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Service   string            `json:"service"`
	Version   string            `json:"version,omitempty"`
	Checks    map[string]string `json:"checks,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
}

// LivenessHandler checks if the server is running and accepting requests
// Always returns 200 OK - no external dependencies required
func (s *Server) LivenessHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s.logger.DebugContext(ctx, "liveness check requested")

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Service:   "vibecheck-mcp",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.ErrorContext(ctx, "failed to encode response", "error", err)
	}

	s.logger.DebugContext(ctx, "liveness check completed", "status", "healthy")
}

// ReadinessHandler checks if the server is ready to handle requests
// Returns 200 OK if storage and langextract are accessible, 503 if not
func (s *Server) ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s.logger.DebugContext(ctx, "readiness check requested")

	// Check storage accessibility
	storageAccessible := s.storageManager.IsAccessible()

	// Check langextract connectivity
	langextractAccessible := s.checkLangExtractConnectivity(ctx)

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Service:   "vibecheck-mcp",
		Checks:    make(map[string]string),
	}

	// Update checks
	if storageAccessible {
		response.Checks["storage"] = "accessible"
	} else {
		response.Checks["storage"] = "inaccessible"
	}

	if langextractAccessible {
		response.Checks["langextract"] = "accessible"
	} else {
		response.Checks["langextract"] = "inaccessible"
	}

	// Determine overall status
	if storageAccessible && langextractAccessible {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			s.logger.ErrorContext(ctx, "failed to encode response", "error", err)
		}
		s.logger.DebugContext(ctx, "readiness check completed", "status", "healthy", "storage", "accessible", "langextract", "accessible")
	} else {
		response.Status = "unhealthy"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			s.logger.ErrorContext(ctx, "failed to encode response", "error", err)
		}
		s.logger.ErrorContext(ctx, "readiness check failed", "status", "unhealthy", "storage", storageAccessible, "langextract", langextractAccessible)
	}
}

// checkLangExtractConnectivity verifies that the LangExtract service is accessible
func (s *Server) checkLangExtractConnectivity(ctx context.Context) bool {
	// Build health check URL
	healthURL := fmt.Sprintf("http://%s/health", s.config.LangExtractHost)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		s.logger.DebugContext(ctx, "failed to create langextract health request", "error", err, "url", healthURL)
		return false
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		s.logger.DebugContext(ctx, "langextract health check failed", "error", err, "url", healthURL)
		return false
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		s.logger.DebugContext(ctx, "langextract health check returned non-OK status", "status", resp.StatusCode, "url", healthURL)
		return false
	}

	s.logger.DebugContext(ctx, "langextract health check succeeded", "url", healthURL)
	return true
}
