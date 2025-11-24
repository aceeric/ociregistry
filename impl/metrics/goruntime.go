// The code in this file is adapted from https://github.com/GilGil1/go-metrics-examples/tree/main based on
// an article here: https://medium.com/cyberark-engineering/golang-monitoring-made-easy-with-version-1-16-df06f7477d75.
// The GitHub LICENSE file is: https://github.com/GilGil1/go-metrics-examples/blob/main/LICENSE
package metrics

import (
	"fmt"
	"runtime/metrics"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// GetSingleMetricFloat is called by Prometheus to produce a metric
func GetSingleMetricFloat(metricName string) float64 {
	sample := make([]metrics.Sample, 1)
	sample[0].Name = metricName
	metrics.Read(sample)
	return getFloat64(sample[0])
}

// addGoRuntimeMetrics retrieves all built-in go runtime metrics and adds them
// to prometheus.
func addGoRuntimeMetrics() {
	metricsMeta := metrics.All()
	for i := range metricsMeta {
		meta := metricsMeta[i]
		opts := getMetricsOptions(metricsMeta[i])
		if meta.Cumulative {
			funcCounter := prometheus.NewCounterFunc(prometheus.CounterOpts(opts), func() float64 {
				return GetSingleMetricFloat(meta.Name)
			})
			prometheus.MustRegister(funcCounter)
		} else {
			funcGauge := prometheus.NewGaugeFunc(prometheus.GaugeOpts(opts), func() float64 {
				return GetSingleMetricFloat(meta.Name)
			})
			prometheus.MustRegister(funcGauge)
		}
	}
}

// getMetricsOptions converts a go metric description into a prometheus
// Opts struct.
func getMetricsOptions(metric metrics.Description) prometheus.Opts {
	tokens := strings.Split(metric.Name, "/")
	if len(tokens) < 2 {
		return prometheus.Opts{}
	}
	nameTokens := strings.Split(tokens[len(tokens)-1], ":")

	metricName := normalizePrometheusName(nameTokens[:2])
	subsystem := getMetricSubsystemName(metric)
	units := nameTokens[1]
	help := fmt.Sprintf("Units:%s, %s", units, metric.Description)

	opts := prometheus.Opts{
		Namespace: tokens[1],
		Subsystem: subsystem,
		Name:      metricName,
		Help:      help,
	}
	return opts
}

// normalizePrometheusName converts a go metric name to a valid
// prometheus metric name
func normalizePrometheusName(name []string) string {
	namea := strings.Join(name, "_")
	return strings.TrimSpace(strings.ReplaceAll(namea, "-", "_"))
}

// getFloat64 returns single values as float64.
func getFloat64(sample metrics.Sample) float64 {
	var floatVal float64
	switch sample.Value.Kind() {
	case metrics.KindUint64:
		floatVal = float64(sample.Value.Uint64())
	case metrics.KindFloat64:
		floatVal = float64(sample.Value.Float64())
	case metrics.KindFloat64Histogram:
		// TEMP - NEED TO FIX THIS
		return medianBucket(sample.Value.Float64Histogram())
	case metrics.KindBad:
		// TODO LOG
	default:
		// TODO LOG
		// panic(fmt.Sprintf("%s: unsupported metric Kind: %v\n", sample.Name, sample.Value.Kind()))
	}
	return floatVal
}

// getMetricSubsystemName parses the metrics description and extracts a
// subsystem.
func getMetricSubsystemName(metric metrics.Description) string {
	tokens := strings.Split(metric.Name, "/")
	if len(tokens) > 3 {
		subsystemTokens := tokens[2 : len(tokens)-1]
		subsystem := strings.Join(subsystemTokens, "_")
		subsystem = strings.ReplaceAll(subsystem, "-", "_")
		return subsystem
	}
	return ""
}

// medianBucket is a bit of a hack to handle go runtime metric histograms. Needs cleanup.
func medianBucket(h *metrics.Float64Histogram) float64 {
	total := uint64(0)
	for _, count := range h.Counts {
		total += count
	}
	thresh := total / 2
	total = 0
	for i, count := range h.Counts {
		total += count
		if total >= thresh {
			return h.Buckets[i]
		}
	}
	// should never happen - TODO log?
	return 0
}
