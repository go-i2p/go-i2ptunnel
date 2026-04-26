package metrics

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// TunnelStatus contains the live state of a single tunnel.
type TunnelStatus struct {
	Name         string `json:"name"`
	ID           string `json:"id"`
	Type         string `json:"type"`
	Status       string `json:"status"`
	Address      string `json:"address,omitempty"`
	Target       string `json:"target,omitempty"`
	LocalAddress string `json:"local_address,omitempty"`
	Error        string `json:"error,omitempty"`
}

// TunnelStatusFunc returns the current status of all tunnels.
// The web UI controller provides this callback.
type TunnelStatusFunc func() []TunnelStatus

// Handler serves HTTP endpoints for metrics, health checks, and status.
type Handler struct {
	Registry   *Registry
	StatusFunc TunnelStatusFunc
}

// NewHandler creates a Handler backed by the given registry and status source.
func NewHandler(registry *Registry, statusFunc TunnelStatusFunc) *Handler {
	return &Handler{
		Registry:   registry,
		StatusFunc: statusFunc,
	}
}

// HandleMetrics writes all metrics in Prometheus text exposition format.
func (h *Handler) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	if err := h.Registry.WritePrometheus(w); err != nil {
		log.WithError(err).Warn("Failed to write Prometheus metrics")
	}
}

// HealthResponse is the JSON body returned by the health check endpoint.
type HealthResponse struct {
	Status         string `json:"status"`
	TunnelsTotal   int    `json:"tunnels_total"`
	TunnelsRunning int    `json:"tunnels_running"`
}

// HandleHealth returns a health check indicating the service is responsive.
// It includes a summary of how many tunnels are running.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	tunnels := h.StatusFunc()

	running := 0
	for _, t := range tunnels {
		if t.Status == "running" {
			running++
		}
	}

	resp := HealthResponse{
		Status:         "ok",
		TunnelsTotal:   len(tunnels),
		TunnelsRunning: running,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// TunnelStatusDetail combines live tunnel state with its metrics snapshot.
type TunnelStatusDetail struct {
	TunnelStatus
	Metrics *MetricSnapshot `json:"metrics,omitempty"`
}

// StatusResponse is the JSON body returned by the status endpoint.
type StatusResponse struct {
	Tunnels []TunnelStatusDetail `json:"tunnels"`
}

// HandleStatus returns detailed status and metrics for all tunnels as JSON.
func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	tunnels := h.StatusFunc()

	details := make([]TunnelStatusDetail, 0, len(tunnels))
	for _, t := range tunnels {
		detail := TunnelStatusDetail{
			TunnelStatus: t,
		}
		if m := h.Registry.Get(t.ID); m != nil {
			snap := m.Snapshot()
			detail.Metrics = &snap
		}
		details = append(details, detail)
	}

	resp := StatusResponse{Tunnels: details}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// WritePrometheus writes all registered metrics in Prometheus text
// exposition format. See https://prometheus.io/docs/instrumenting/exposition_formats/
func (r *Registry) WritePrometheus(w io.Writer) error {
	snapshots := r.Snapshots()
	return writePrometheusMetrics(w, snapshots)
}

// prometheusMetric defines a single metric family for Prometheus output.
type prometheusMetric struct {
	name string
	help string
	typ  string // "counter" or "gauge"
	val  func(MetricSnapshot) int64
}

// counterMetrics are cumulative totals.
var counterMetrics = []prometheusMetric{
	{
		"i2ptunnel_connections_accepted_total", "Total accepted connections", "counter",
		func(s MetricSnapshot) int64 { return s.ConnectionsAccepted },
	},
	{
		"i2ptunnel_connections_failed_total", "Total failed connection attempts", "counter",
		func(s MetricSnapshot) int64 { return s.ConnectionsFailed },
	},
	{
		"i2ptunnel_bytes_received_total", "Total bytes received", "counter",
		func(s MetricSnapshot) int64 { return s.BytesIn },
	},
	{
		"i2ptunnel_bytes_sent_total", "Total bytes sent", "counter",
		func(s MetricSnapshot) int64 { return s.BytesOut },
	},
	{
		"i2ptunnel_errors_total", "Total errors encountered", "counter",
		func(s MetricSnapshot) int64 { return s.ErrorCount },
	},
	{
		"i2ptunnel_rate_limit_hits_total", "Total rate limit rejections", "counter",
		func(s MetricSnapshot) int64 { return s.RateLimitHits },
	},
	{
		"i2ptunnel_filter_blocks_total", "Total filter blocks", "counter",
		func(s MetricSnapshot) int64 { return s.FilterBlocks },
	},
}

// gaugeMetrics are point-in-time values.
var gaugeMetrics = []prometheusMetric{
	{
		"i2ptunnel_active_connections", "Currently active connections", "gauge",
		func(s MetricSnapshot) int64 { return s.ActiveConnections },
	},
}

func writePrometheusMetrics(w io.Writer, snapshots []MetricSnapshot) error {
	allMetrics := make([]prometheusMetric, 0, len(counterMetrics)+len(gaugeMetrics))
	allMetrics = append(allMetrics, counterMetrics...)
	allMetrics = append(allMetrics, gaugeMetrics...)
	for _, m := range allMetrics {
		if err := writeMetricFamily(w, m, snapshots); err != nil {
			return err
		}
	}
	return writeUptimeMetric(w, snapshots)
}

// writeUptimeMetric writes the i2ptunnel_uptime_seconds gauge family.
func writeUptimeMetric(w io.Writer, snapshots []MetricSnapshot) error {
	if _, err := fmt.Fprintf(w, "# HELP i2ptunnel_uptime_seconds Tunnel uptime in seconds\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "# TYPE i2ptunnel_uptime_seconds gauge\n"); err != nil {
		return err
	}
	for _, s := range snapshots {
		labels := formatLabels(s.Name, s.ID, s.Type)
		if _, err := fmt.Fprintf(w, "i2ptunnel_uptime_seconds{%s} %.3f\n", labels, s.UptimeSeconds); err != nil {
			return err
		}
	}
	return nil
}

// writeMetricFamily writes the HELP, TYPE, and per-tunnel sample lines for one metric.
func writeMetricFamily(w io.Writer, m prometheusMetric, snapshots []MetricSnapshot) error {
	if _, err := fmt.Fprintf(w, "# HELP %s %s\n", m.name, m.help); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "# TYPE %s %s\n", m.name, m.typ); err != nil {
		return err
	}
	for _, s := range snapshots {
		labels := formatLabels(s.Name, s.ID, s.Type)
		if _, err := fmt.Fprintf(w, "%s{%s} %d\n", m.name, labels, m.val(s)); err != nil {
			return err
		}
	}
	return nil
}

// formatLabels formats Prometheus label key-value pairs.
func formatLabels(name, id, tunnelType string) string {
	return fmt.Sprintf(
		`name="%s",id="%s",type="%s"`,
		escapeLabelValue(name),
		escapeLabelValue(id),
		escapeLabelValue(tunnelType),
	)
}

// escapeLabelValue escapes backslash, double-quote, and newline
// per the Prometheus text format specification.
func escapeLabelValue(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}
