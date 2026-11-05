package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

func requireKey(expected string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := r.Header.Get("X-Internal-API-Key")
		if expected == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid internal api key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readyHandler(store interface{ Ready() error }) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if err := store.Ready(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "down", "checks": map[string]string{"storage": "down"}})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "up", "checks": map[string]string{"storage": "up"}})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (writer *responseRecorder) WriteHeader(status int) {
	if writer.status == 0 {
		writer.status = status
	}
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *responseRecorder) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	written, err := writer.ResponseWriter.Write(body)
	writer.bytes += written
	return written, err
}

func (writer *responseRecorder) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

func logging(next http.Handler, metrics *httpMetrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" || len(requestID) > 128 {
			requestID = strconv.FormatInt(time.Now().UnixNano(), 36)
		}
		w.Header().Set("X-Request-ID", requestID)
		start := time.Now()
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		duration := time.Since(start)
		metrics.observe(r.Method, status, duration, recorder.bytes)
		slog.Info("request", "request_id", requestID, "method", r.Method, "path", r.URL.Path, "status", status, "bytes", recorder.bytes, "duration", duration.String(), "bytes_per_second", throughput(recorder.bytes, duration))
	})
}

func throughput(bytes int, duration time.Duration) int64 {
	if bytes < 1 || duration <= 0 {
		return 0
	}
	return int64(float64(bytes) / duration.Seconds())
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
