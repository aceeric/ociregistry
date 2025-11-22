package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// These are the metrics functions exposed by the package. By default they are all
// NOP functions to minimize overhead when metrics are not enabled. The 'initAllMetrics'
// function initializes these with functions having implementations if metrics are
// enabled.

var IncCachedPullsByNs withLabel = func(string) {}
var IncUpstreamPullsByNs withLabel = func(string) {}
var IncManifestPulls noLabel = func() {}
var IncBlobPulls noLabel = func() {}
var DeltaBlobBytesOnDisk delta = func(float64) {}
var DeltaManifestBytesOnDisk delta = func(float64) {}
var DeltaCachedManifestCount delta = func(float64) {}
var DeltaCachedBlobCount delta = func(float64) {}
var IncV2ApiEndpointHits noLabel = func() {}
var IncApiErrorResults noLabel = func() {}

// metricAction carries a metric update through the channel to a function that uses the
// struct values to make calls to the Prometheus library.
//type metricAction struct {
//	name     string
//	value    float64
//	labelval string
//}

type withLabel func(string)
type noLabel func()
type delta func(float64)

// "ns" below refers to the upstream namespace, like "docker.io" or "ghcr.io"
const (
	cached_pulls_by_ns_total     = "cached_pulls_by_ns_total"
	upstream_pulls_by_ns         = "upstream_pulls_by_ns"
	manifest_pulls_total         = "manifest_pulls_total"
	blob_pulls_total             = "blob_pulls_total"
	blob_bytes_on_disk_total     = "blob_bytes_on_disk_total"
	manifest_bytes_on_disk_total = "manifest_bytes_on_disk_total"
	cached_manifest_count        = "cached_manifest_count"
	cached_blob_count            = "cached_blob_count"
	v2_api_endpoint_hits_total   = "v2_api_endpoint_hits_total"
	api_errors_total             = "api_errors_total"
	ns_label                     = "ns"
)

// metricsChan is used by this go file to decouple callers setting metrics from the underlying
// Prometheus metrics library. The purpose is to reduce synchronization overhead from that
// library.
// var metricsChan chan metricAction

// these are the actual Prometheus metrics objects initialized by the 'initAllMetrics' function

var cachedPullsByNsTotal *prometheus.CounterVec
var upstreamPullsByNsTotal *prometheus.CounterVec
var manifestPullsTotal prometheus.Counter
var blobPullsTotal prometheus.Counter
var blobBytesOnDiskTotal prometheus.Gauge
var manifestBytesOnDiskTotal prometheus.Gauge
var cachedManifestCount prometheus.Gauge
var cachedBlobCount prometheus.Gauge
var v2ApiEndpointHitsTotal prometheus.Counter
var apiErrorsTotal prometheus.Counter

// addOciregistryMetrics initializes the metrics specific to the ociregistry server.
func addOciregistryMetrics() {
	initAllMetrics()
	//metricsChan = make(chan metricAction, 10000)
	//go func() {
	//	for {
	//		ma, more := <-metricsChan
	//		if more {
	//			handleMetric(ma)
	//		} else {
	//			// channel closed
	//			return
	//		}
	//	}
	//}()
}

// handleMetric maps the metric name in the passed MetricAction to one of the Prometheus
// metric objects (Counter, CounterVec, Gauge, or GaugeVec) and then invokes the operation
// (add or set) that is appropriate for the metric.
//func handleMetric(ma metricAction) {
//	switch ma.name {
//	case cached_pulls_by_ns_total:
//		cachedPullsByNsTotal.With(prometheus.Labels{ns_label: ma.labelval}).Add(ma.value)
//	case upstream_pulls_by_ns:
//		uncachedPullsByNsTotal.With(prometheus.Labels{ns_label: ma.labelval}).Add(ma.value)
//	case manifest_pulls_total:
//		manifestPullsTotal.Add(ma.value)
//	case blob_bytes_on_disk_total:
//		blobBytesOnDiskTotal.Add(ma.value)
//	case manifest_bytes_on_disk_total:
//		manifestBytesOnDiskTotal.Add(ma.value)
//	case cached_manifest_count:
//		cachedManifestCount.Add(ma.value)
//	case cached_blob_count:
//		cachedBlobCount.Add(ma.value)
//	case v2_api_endpoint_hits_total:
//		v2ApiEndpointHitsTotal.Add(ma.value)
//	case api_errors_total:
//		apiErrorsTotal.Add(ma.value)
//	default:
//		log.Errorf("unknown metric: %s", ma.name)
//	}
//}

// recordMetric is called to record a metric value. The passed values are simply written to the
// 'metricsChan' channel and then the function returns. The goal is to minimize synchronization overhead.
// (The Prometheus library uses the go 'atomic' library in its internals which introduces a tiny bit of
// additional sycnhronization.) By writing the metric to a channel and then immediately exiting, we
// minimize synchronization. (TODO test alternate approach without channel.)
//
// The risk is that if the imcoming metrics exceed the buffer size then they are discarded. And,
// of course, go synchronizes channel access so we can't escape some synchronization overhead.
//func recordMetric(name string, value float64, labelval string) {
//	select {
//	case metricsChan <- metricAction{
//		name:     name,
//		value:    value,
//		labelval: labelval,
//	}:
//	default:
//		log.Debugf("ocimetrics channel full, metric discarded: %s", name)
//	}
//
//}

// initAllMetrics creates all the ociregistry metrics and registers them with the
// prometheus library. It also assigns a function to actually implement the metric.
// Unless *this* function is called, all the metric functions exposed by the package will
// be NOP functions.
func initAllMetrics() {
	cachedPullsByNsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name:      cached_pulls_by_ns_total,
			Namespace: "ociregistry",
			Help:      "Total pulls of cached images by namespace",
		},
		[]string{ns_label},
	)
	IncCachedPullsByNs = func(ns string) {
		//recordMetric(cached_pulls_by_ns_total, 1, ns)
		cachedPullsByNsTotal.With(prometheus.Labels{ns_label: ns}).Add(1)
	}

	///
	upstreamPullsByNsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name:      upstream_pulls_by_ns,
			Namespace: "ociregistry",
			Help:      "Total pulls of un-cached images by namespace",
		},
		[]string{ns_label},
	)
	IncUpstreamPullsByNs = func(ns string) {
		//recordMetric(upstream_pulls_by_ns, 1, ns)
		upstreamPullsByNsTotal.With(prometheus.Labels{ns_label: ns}).Add(1)
	}

	///
	manifestPullsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name:      manifest_pulls_total,
			Namespace: "ociregistry",
			Help:      "Total count of all manifest pulls",
		},
	)
	IncManifestPulls = func() {
		//recordMetric(manifest_pulls_total, 1, "")
		manifestPullsTotal.Add(1)
	}

	///
	blobPullsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name:      blob_pulls_total,
			Namespace: "ociregistry",
			Help:      "Total count of all blob pulls",
		},
	)
	IncBlobPulls = func() {
		//recordMetric(blob_pulls_total, 1, "")
		blobPullsTotal.Add(1)
	}

	///
	blobBytesOnDiskTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name:      blob_bytes_on_disk_total,
			Namespace: "ociregistry",
			Help:      "Total blob bytes on disk (independent of file system overhead)",
		},
	)
	DeltaBlobBytesOnDisk = func(delta float64) {
		//recordMetric(blob_bytes_on_disk_total, delta, "")
		blobBytesOnDiskTotal.Add(delta)
	}

	///
	manifestBytesOnDiskTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name:      manifest_bytes_on_disk_total,
			Namespace: "ociregistry",
			Help:      "Total manifest bytes on disk (independent of file system overhead)",
		},
	)
	DeltaManifestBytesOnDisk = func(delta float64) {
		//recordMetric(manifest_bytes_on_disk_total, delta, "")
		manifestBytesOnDiskTotal.Add(delta)
	}

	///
	manifestBytesOnDiskTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name:      cached_manifest_count,
			Namespace: "ociregistry",
			Help:      "Total number of manifests cached in memory by tag, digest, and latest",
		},
	)
	DeltaCachedManifestCount = func(delta float64) {
		//recordMetric(cached_manifest_count, delta, "")
		cachedManifestCount.Add(delta)
	}

	///
	blobBytesOnDiskTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name:      cached_blob_count,
			Namespace: "ociregistry",
			Help:      "Total number of cached blobs",
		},
	)
	DeltaCachedBlobCount = func(delta float64) {
		//recordMetric(cached_blob_count, delta, "")
		cachedBlobCount.Add(delta)
	}

	///
	v2ApiEndpointHitsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name:      v2_api_endpoint_hits_total,
			Namespace: "ociregistry",
			Help:      "Totals calls to the v2 oci distribution api spec endpoints implemented by the server",
		},
	)
	IncV2ApiEndpointHits = func() {
		//recordMetric(v2_api_endpoint_hits_total, 1, "")
		v2ApiEndpointHitsTotal.Add(1)
	}

	///
	apiErrorsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name:      api_errors_total,
			Namespace: "ociregistry",
			Help:      "Totals calls to the v2 oci distribution api spec endpoints that resulted in errors (not found, internal server error)",
		},
	)
	IncApiErrorResults = func() {
		//recordMetric(api_errors_total, 1, "")
		apiErrorsTotal.Add(1)
	}
}
