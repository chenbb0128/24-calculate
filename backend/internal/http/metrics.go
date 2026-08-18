package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

type requestMetric struct {
	Count       uint64
	ErrorCount  uint64
	DurationSum float64
}

// Metrics is a deliberately small Prometheus-compatible collector. It keeps
// endpoint labels bounded to Gin's registered route pattern, never a raw URL
// containing user-controlled ids.
type Metrics struct {
	started  time.Time
	inFlight atomic.Int64
	mu       sync.Mutex
	requests map[string]*requestMetric
}

func NewMetrics() *Metrics {
	return &Metrics{started: time.Now().UTC(), requests: make(map[string]*requestMetric)}
}

func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m == nil {
			c.Next()
			return
		}
		started := time.Now()
		m.inFlight.Add(1)
		c.Next()
		m.inFlight.Add(-1)
		route := c.FullPath()
		if route == "" {
			route = "unknown"
		}
		key := c.Request.Method + " " + route
		m.mu.Lock()
		item := m.requests[key]
		if item == nil {
			item = &requestMetric{}
			m.requests[key] = item
		}
		item.Count++
		if c.Writer.Status() >= http.StatusInternalServerError {
			item.ErrorCount++
		}
		item.DurationSum += time.Since(started).Seconds()
		m.mu.Unlock()
	}
}

func (m *Metrics) Handler(c *gin.Context) {
	if m == nil {
		c.String(http.StatusServiceUnavailable, "metrics unavailable\n")
		return
	}
	var builder strings.Builder
	builder.WriteString("# HELP twenty_four_calculate_uptime_seconds Process uptime in seconds.\n")
	builder.WriteString("# TYPE twenty_four_calculate_uptime_seconds gauge\n")
	fmt.Fprintf(&builder, "twenty_four_calculate_uptime_seconds %.3f\n", time.Since(m.started).Seconds())
	builder.WriteString("# HELP twenty_four_calculate_http_requests_in_flight Current HTTP requests.\n")
	builder.WriteString("# TYPE twenty_four_calculate_http_requests_in_flight gauge\n")
	fmt.Fprintf(&builder, "twenty_four_calculate_http_requests_in_flight %d\n", m.inFlight.Load())
	builder.WriteString("# HELP twenty_four_calculate_http_requests_total HTTP requests by method and route.\n")
	builder.WriteString("# TYPE twenty_four_calculate_http_requests_total counter\n")
	builder.WriteString("# HELP twenty_four_calculate_http_errors_total HTTP 5xx responses by method and route.\n")
	builder.WriteString("# TYPE twenty_four_calculate_http_errors_total counter\n")
	builder.WriteString("# HELP twenty_four_calculate_http_request_duration_seconds_sum HTTP request duration sum.\n")
	builder.WriteString("# TYPE twenty_four_calculate_http_request_duration_seconds_sum counter\n")
	m.mu.Lock()
	for key, item := range m.requests {
		parts := strings.SplitN(key, " ", 2)
		method, route := parts[0], "unknown"
		if len(parts) == 2 {
			route = parts[1]
		}
		route = strings.ReplaceAll(strings.ReplaceAll(route, `\`, `\\`), `"`, `\"`)
		fmt.Fprintf(&builder, "twenty_four_calculate_http_requests_total{method=\"%s\",route=\"%s\"} %d\n", method, route, item.Count)
		fmt.Fprintf(&builder, "twenty_four_calculate_http_errors_total{method=\"%s\",route=\"%s\"} %d\n", method, route, item.ErrorCount)
		fmt.Fprintf(&builder, "twenty_four_calculate_http_request_duration_seconds_sum{method=\"%s\",route=\"%s\"} %.6f\n", method, route, item.DurationSum)
	}
	m.mu.Unlock()
	c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(builder.String()))
}
