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

// Prometheus metrics objects

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

// addOciregistryMetrics creates all the ociregistry metrics and registers them with the
// prometheus library. It also assigns a function to actually implement the metric.
// Unless this function is called, all the metric functions exposed by the package
// will be NOP functions.
func addOciregistryMetrics() {
	cachedPullsByNsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name:      cached_pulls_by_ns_total,
			Namespace: "ociregistry",
			Help:      "Total pulls of cached images by namespace",
		},
		[]string{ns_label},
	)
	IncCachedPullsByNs = func(ns string) {
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
		manifestBytesOnDiskTotal.Add(delta)
	}

	///
	cachedManifestCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name:      cached_manifest_count,
			Namespace: "ociregistry",
			Help:      "Total number of manifests cached in memory by tag, digest, and latest",
		},
	)
	DeltaCachedManifestCount = func(delta float64) {
		cachedManifestCount.Add(delta)
	}

	///
	cachedBlobCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name:      cached_blob_count,
			Namespace: "ociregistry",
			Help:      "Total number of cached blobs",
		},
	)
	DeltaCachedBlobCount = func(delta float64) {
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
		apiErrorsTotal.Add(1)
	}
}
