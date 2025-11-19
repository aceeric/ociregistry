package metrics

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// initMetrics creates all the Prometheus metrics and registers them for
// availability at the passed port number under the '/metrics' path. Then it
// starts an HTTP server to serve the metrics.
func InitMetrics(port int) {
	addGoRuntimeMetrics()
	addOciregistryMetrics()
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
