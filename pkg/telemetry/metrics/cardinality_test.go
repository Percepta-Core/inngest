package metrics

import (
	"context"
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

const labelCombinations = 2500

func addCombinations(t *testing.T, meter otelmetric.Meter, name string) {
	t.Helper()
	counter, err := meter.Int64Counter(name)
	require.NoError(t, err)
	for i := 0; i < labelCombinations; i++ {
		counter.Add(context.Background(), 1, otelmetric.WithAttributes(
			attribute.String("account_id", fmt.Sprintf("acct-%d", i%50)),
			attribute.String("function_id", fmt.Sprintf("fn-%d", i)),
		))
	}
}

func TestPrometheusProviderKeepsEveryLabelCombination(t *testing.T) {
	m, err := NewPrometheusMeterProvider(context.Background(), "cardinality-test")
	require.NoError(t, err)
	defer m.Shutdown()

	addCombinations(t, m.Provider.Meter("cardinality-test"), "cardinality_prom")

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	var series int
	for _, family := range families {
		if family.GetName() != "cardinality_prom_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			series++
			for _, label := range metric.GetLabel() {
				require.NotEqual(t, "otel_metric_overflow", label.GetName())
			}
		}
	}
	require.Equal(t, labelCombinations, series)
}

func TestSDKDefaultLimitWouldCollapseLabels(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() { _ = provider.Shutdown(context.Background()) }()

	addCombinations(t, provider.Meter("cardinality-control"), "cardinality_control")

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	var points int
	var overflow bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			sum, ok := m.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			for _, dp := range sum.DataPoints {
				points++
				if v, found := dp.Attributes.Value("otel.metric.overflow"); found && v.AsBool() {
					overflow = true
				}
			}
		}
	}
	require.Less(t, points, labelCombinations)
	require.True(t, overflow)
}
