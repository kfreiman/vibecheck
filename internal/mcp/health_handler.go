package mcp

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/kfreiman/vibecheck/internal/storage"
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
func LivenessHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slog.Default().DebugContext(ctx, "liveness check requested")

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Service:   "vibecheck-mcp",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

	slog.Default().DebugContext(ctx, "liveness check completed", "status", "healthy")
}

// LivenessHandlerWithLogger checks if the server is running and accepts a logger for testability
func LivenessHandlerWithLogger(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {
	ctx := r.Context()
	logger.DebugContext(ctx, "liveness check requested")

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Service:   "vibecheck-mcp",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

	logger.DebugContext(ctx, "liveness check completed", "status", "healthy")
}

// ReadinessHandler checks if the server is ready to handle requests
// Returns 200 OK if storage is accessible, 503 if not
func ReadinessHandler(w http.ResponseWriter, r *http.Request, storageManager *storage.StorageManager) {
	ctx := r.Context()
	slog.Default().DebugContext(ctx, "readiness check requested")

	// Check storage accessibility
	storageAccessible := storageManager.IsAccessible()

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
		json.NewEncoder(w).Encode(response)
		slog.Default().DebugContext(ctx, "readiness check completed", "status", "healthy", "storage", "accessible")
	} else {
		response.Status = "unhealthy"
		response.Checks["storage"] = "inaccessible"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(response)
		slog.Default().ErrorContext(ctx, "readiness check failed", "status", "unhealthy", "storage", "inaccessible")
	}
}

// ReadinessHandlerWithLogger checks if the server is ready and accepts a logger for testability
func ReadinessHandlerWithLogger(w http.ResponseWriter, r *http.Request, storageManager *storage.StorageManager, logger *slog.Logger) {
	ctx := r.Context()
	logger.DebugContext(ctx, "readiness check requested")

	// Check storage accessibility
	storageAccessible := storageManager.IsAccessible()

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
		json.NewEncoder(w).Encode(response)
		logger.DebugContext(ctx, "readiness check completed", "status", "healthy", "storage", "accessible")
	} else {
		response.Status = "unhealthy"
		response.Checks["storage"] = "inaccessible"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(response)
		logger.ErrorContext(ctx, "readiness check failed", "status", "unhealthy", "storage", "inaccessible")
	}
}

// ReadinessHandlerFunc creates a handler function with storage manager dependency
func ReadinessHandlerFunc(storageManager *storage.StorageManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ReadinessHandler(w, r, storageManager)
	}
}

// ReadinessHandlerFuncWithLogger creates a handler function with storage manager and logger dependency
func ReadinessHandlerFuncWithLogger(storageManager *storage.StorageManager, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ReadinessHandlerWithLogger(w, r, storageManager, logger)
	}
}
