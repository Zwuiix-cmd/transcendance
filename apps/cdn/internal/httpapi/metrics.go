package httpapi

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

type metricKey struct {
	method string
	status int
}

type metricValue struct {
	count    uint64
	duration time.Duration
	bytes    uint64
}

type httpMetrics struct {
	mu     sync.RWMutex
	values map[metricKey]metricValue
}

func newMetrics() *httpMetrics {
	return &httpMetrics{values: make(map[metricKey]metricValue)}
}

func (metrics *httpMetrics) observe(method string, status int, duration time.Duration, bytes int) {
	metrics.mu.Lock()
	value := metrics.values[metricKey{method: method, status: status}]
	value.count++
	value.duration += duration
	if bytes > 0 {
		value.bytes += uint64(bytes)
	}
	metrics.values[metricKey{method: method, status: status}] = value
	metrics.mu.Unlock()
}

func (metrics *httpMetrics) ServeHTTP(writer http.ResponseWriter, _ *http.Request) {
	metrics.mu.RLock()
	keys := make([]metricKey, 0, len(metrics.values))
	for key := range metrics.values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(left, right int) bool {
		if keys[left].method == keys[right].method {
			return keys[left].status < keys[right].status
		}
		return keys[left].method < keys[right].method
	})
	writer.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintln(writer, "# HELP cdn_http_requests_total Total HTTP requests handled by the CDN.")
	_, _ = fmt.Fprintln(writer, "# TYPE cdn_http_requests_total counter")
	_, _ = fmt.Fprintln(writer, "# HELP cdn_http_request_duration_seconds_sum Total request duration in seconds.")
	_, _ = fmt.Fprintln(writer, "# TYPE cdn_http_request_duration_seconds_sum counter")
	_, _ = fmt.Fprintln(writer, "# HELP cdn_http_response_bytes_total Total response bytes served by the CDN.")
	_, _ = fmt.Fprintln(writer, "# TYPE cdn_http_response_bytes_total counter")
	for _, key := range keys {
		value := metrics.values[key]
		labels := fmt.Sprintf(`method="%s",status="%d"`, key.method, key.status)
		_, _ = fmt.Fprintf(writer, "cdn_http_requests_total{%s} %d\n", labels, value.count)
		_, _ = fmt.Fprintf(writer, "cdn_http_request_duration_seconds_sum{%s} %f\n", labels, value.duration.Seconds())
		_, _ = fmt.Fprintf(writer, "cdn_http_response_bytes_total{%s} %d\n", labels, value.bytes)
	}
	metrics.mu.RUnlock()
}
