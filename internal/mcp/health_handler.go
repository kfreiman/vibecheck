package mcp

import (
	"encoding/json"
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
// Returns 200 OK if storage is accessible, 503 if not
func (s *Server) ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s.logger.DebugContext(ctx, "readiness check requested")

	// Check storage accessibility
	storageAccessible := s.storageManager.IsAccessible()

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Service:   "vibecheck-mcp",
		Checks:    make(map[string]string),
	}

	if storageAccessible {
		response.Checks["storage"] = "accessible"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			s.logger.ErrorContext(ctx, "failed to encode response", "error", err)
		}
		s.logger.DebugContext(ctx, "readiness check completed", "status", "healthy", "storage", "accessible")
	} else {
		response.Status = "unhealthy"
		response.Checks["storage"] = "inaccessible"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			s.logger.ErrorContext(ctx, "failed to encode response", "error", err)
		}
		s.logger.ErrorContext(ctx, "readiness check failed", "status", "unhealthy", "storage", "inaccessible")
	}
}
