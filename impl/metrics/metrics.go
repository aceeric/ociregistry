package metrics

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// initMetrics initializes metrics. If the passed port is zero, no action is taken. Otherwise,
// the function creates all the go runtime and ociregistry metrics and registers them for availability
// at the passed port number under the '/metrics' path. Then it starts an HTTP server to
// serve the metrics.
func InitMetrics(port int) {
	if port == 0 {
		return
	}
	addGoRuntimeMetrics()
	addOciregistryMetrics()
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
