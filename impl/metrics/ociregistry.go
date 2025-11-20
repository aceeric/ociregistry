package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// cached pulls by namespace (rate)
// un-cached pulls by namespace (rate)
// total pulls (rate)
// blob bytes on disk (total)
// manifest bytes on disk (total)
// cached manifest count (total)
// cached blob count (total)
// v2 api endpoint hits (rate)
// cmd endpoint hits (rate)
// total api error results (rate)

// MetricAction is a generic way for callers to transmit metric values without knowing
// that the metrics are implemented by Prometheus. The caller *does* need to understand
// the type of metric. For example, if a counter then the value they are passing is
// what is *add*ed to the counter. If a gauge, then the passed value is a cumulative value
// that the caller is maintaining themselves which is *set* into the gauge. E.g.:
//
//	var myMetric = MetricAction{
//		name:     "cached_pulls_by_namespace",
//		value:    1,
//		labelval: "docker.io",
//	}
//
// In the example, the caller knows this is a counter and so the value of 1 will be added.
type MetricAction struct {
	name     string
	value    float64
	labelval string
}

func IncCachedPullsByNs(ns string) MetricAction {
	return MetricAction{
		name:     cached_pulls_by_namespace,
		value:    1,
		labelval: ns,
	}
}

const (
	cached_pulls_by_namespace = "cached_pulls_by_namespace"
	ns_label                  = "ns"
)

// metricsChan is used by this go file to decouple callers setting metrics from the underlying
// Prometheus metrics library. The purpose is to not incur any synchronization overhead from that
// library.
var metricsChan chan MetricAction

// these are the actual Prometheus metrics objects initialized by the 'initAllMetrics' function

var cachedPullsByNamespace *prometheus.CounterVec

// addOciregistryMetrics first checks whether metrics are enabled. If not enabled, then no action
// is taken. Otherwise, all the metrics are initialized and then the function starts a goroutine
// to listen on a buffered channel. The channel will be filled by ociregistry code recording metric
// values. As a metric value is pulled from the channel it is sent to the 'handleMetric' function.
func addOciregistryMetrics() {
	if !metricsEnabled {
		return
	}
	initAllMetrics()
	metricsChan = make(chan MetricAction, 1000)
	go func() {
		for {
			ma, more := <-metricsChan
			if more {
				handleMetric(ma)
			} else {
				return
			}
		}
	}()

}

// handleMetric maps the metric name in the passed MetricAction to one of the Prometheus
// metric objects (Counter, CounterVec, Gauge, or GaugeVec) and then invokes the operation
// (add or set) that is appropriate for the metric.
func handleMetric(ma MetricAction) {
	switch ma.name {
	case cached_pulls_by_namespace:
		cachedPullsByNamespace.With(prometheus.Labels{ns_label: ma.labelval}).Add(ma.value)
	}
}

// RecordMetric is called to record a metric value. The passed MetricAction is simply written to
// the metricsChan channel and then the function returns. This enables the ociregistry to record
// metrics with no synchronizations. (The Prometheus library uses the atomic library in its internals
// which introduces a tiny bit of additional sycnhronization.) By writing the metric to a channel and
// then immediately exiting, we avoid any synchronization. The risk is that if the imcoming metrics
// exceed the buffer size then they are discarded.
func RecordMetric(ma MetricAction) {
	if !metricsEnabled {
		return
	}
	select {
	case metricsChan <- ma:
	default:
		// channel is full so discard the metric
		// TODO log?
	}
}

// initAllMetrics creates all the ociregistry metrics and registers them with the
// prometheus library.
func initAllMetrics() {
	cachedPullsByNamespace = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name:      cached_pulls_by_namespace,
			Namespace: "ociregistry",
			Help:      "Pulls of cached images by namespace",
		},
		[]string{ns_label},
	)
}
