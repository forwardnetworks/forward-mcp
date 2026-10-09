package httpserver

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version,omitempty"`
}

// ReadinessResponse represents the readiness check response
type ReadinessResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Checks    map[string]string `json:"checks"`
}

// HealthHandler returns a handler for health checks
// Health check indicates if the server process is running (always returns 200 if reachable)
func HealthHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now(),
			Version:   version,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

// ReadinessHandler returns a handler for readiness checks
// Readiness check indicates if the server is ready to accept traffic
// In the future, this could check database connections, API availability, etc.
func ReadinessHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checks := make(map[string]string)
		allReady := true

		// Add readiness checks here as needed
		// For now, we're always ready if we're running
		checks["server"] = "ready"

		status := "ready"
		statusCode := http.StatusOK
		if !allReady {
			status = "not_ready"
			statusCode = http.StatusServiceUnavailable
		}

		response := ReadinessResponse{
			Status:    status,
			Timestamp: time.Now(),
			Checks:    checks,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}
}
