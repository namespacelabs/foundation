package tracing

import (
	"context"
	"fmt"
	sync "sync"
	"time"

	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
)

var (
	mu            sync.Mutex
	meterProvider *metric.MeterProvider
)

func ProvideMeterProvider(ctx context.Context, _ *NoArgs, deps ExtensionDeps) (*metric.MeterProvider, error) {
	mu.Lock()
	existing := meterProvider
	mu.Unlock()
	if existing != nil {
		return existing, nil
	}

	res, err := CreateResource(ctx, deps.ServerInfo, consumeDetectors())
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	prom, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	opts := []metric.Option{metric.WithResource(res), metric.WithReader(prom)}

	exports := consumeMetricsExporters()
	if len(exports) == 0 {
		opts = append(opts, metric.WithReader(metric.NewManualReader()))
	} else {
		for _, exporter := range exports {
			opts = append(opts, metric.WithReader(metric.NewPeriodicReader(exporter, metric.WithInterval(10*time.Second))))
		}
	}

	p := metric.NewMeterProvider(opts...)

	mu.Lock()
	meterProvider = p
	mu.Unlock()
	return p, nil
}
