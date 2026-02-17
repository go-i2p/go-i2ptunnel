// Package metrics provides monitoring and observability for I2P tunnels.
//
// It tracks operational metrics such as connection counts, bytes transferred,
// error rates, rate limit hits, and filter blocks. Metrics are exposed in
// three formats:
//
//   - Prometheus text exposition format at /metrics
//   - JSON health check at /healthz
//   - JSON detailed status at /api/status
//
// # Usage
//
// Tunnels register with the global DefaultRegistry or a custom Registry.
// The Handler serves all three HTTP endpoints.
//
//	registry := metrics.NewRegistry()
//	m := registry.Register("my-tunnel", "my-tunnel-id", "tcpclient")
//	m.RecordStart()
//	m.RecordConnection()
//	m.RecordBytesIn(1024)
//	m.RecordDisconnection()
//
//	handler := metrics.NewHandler(registry, statusFunc)
//	http.Handle("/metrics", http.HandlerFunc(handler.HandleMetrics))
//	http.Handle("/healthz", http.HandlerFunc(handler.HandleHealth))
//	http.Handle("/api/status", http.HandlerFunc(handler.HandleStatus))
package metrics
