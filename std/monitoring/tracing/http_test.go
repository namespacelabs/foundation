package tracing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestHTTPClientCancellationMetrics(t *testing.T) {
	started := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/error" {
			w.WriteHeader(http.StatusInternalServerError)
		} else if r.URL.Path == "/cancel" {
			started <- struct{}{}
			<-r.Context().Done()
		}
	}))
	defer server.Close()

	cause := errors.New("caller stopped waiting")
	for _, tc := range []struct {
		name     string
		url      string
		cancel   func(context.Context) context.Context
		wantErr  error
		canceled bool
		status   int64
	}{
		{name: "success", url: server.URL, status: 200},
		{name: "server error", url: server.URL + "/error", status: 500},
		{name: "untyped transport error", url: "ftp://example.invalid"},
		{
			name: "caller cancellation", url: server.URL + "/cancel", wantErr: context.Canceled, canceled: true,
			cancel: func(ctx context.Context) context.Context {
				ctx, cancel := context.WithCancel(ctx)
				go func() {
					select {
					case <-started:
					case <-ctx.Done():
					}
					cancel()
				}()
				return ctx
			},
		},
		{
			name: "caller cancellation with cause", url: server.URL, wantErr: cause, canceled: true,
			cancel: func(ctx context.Context) context.Context {
				ctx, cancel := context.WithCancelCause(ctx)
				cancel(cause)
				return ctx
			},
		},
		{
			name: "deadline", url: server.URL, wantErr: context.DeadlineExceeded,
			cancel: func(ctx context.Context) context.Context {
				ctx, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
				defer cancel()
				return ctx
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := metric.NewManualReader()
			provider := metric.NewMeterProvider(metric.WithReader(reader))
			defer func() { assert.NoError(t, provider.Shutdown(t.Context())) }()
			inherited := &otelhttp.Labeler{}
			inherited.Add(attribute.String("test.inherited", "kept"))
			parent := otelhttp.ContextWithLabeler(t.Context(), inherited)
			ctx := parent
			if tc.cancel != nil {
				ctx = tc.cancel(ctx)
			}
			transport := (HttpClientProvider{}).WrapTransport(nil,
				otelhttp.WithMeterProvider(provider),
				otelhttp.WithMetricAttributesFn(func(*http.Request) []attribute.KeyValue {
					return []attribute.KeyValue{attribute.String("test.option", "kept")}
				}),
			)
			req, err := http.NewRequestWithContext(ctx, http.MethodHead, tc.url, nil)
			if !assert.NoError(t, err) {
				return
			}
			res, err := transport.RoundTrip(req)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else if tc.status == 0 {
				assert.ErrorContains(t, err, "unsupported protocol scheme")
			} else if assert.NoError(t, err) && assert.NotNil(t, res) {
				assert.Equal(t, int(tc.status), res.StatusCode)
				assert.NoError(t, res.Body.Close())
			}
			assert.Equal(t, []attribute.KeyValue{attribute.String("test.inherited", "kept")}, inherited.Get())

			var metrics metricdata.ResourceMetrics
			assert.NoError(t, reader.Collect(t.Context(), &metrics))
			var points []metricdata.HistogramDataPoint[float64]
			for _, scope := range metrics.ScopeMetrics {
				for _, m := range scope.Metrics {
					if m.Name == "http.client.request.duration" {
						points = append(points, m.Data.(metricdata.Histogram[float64]).DataPoints...)
					}
				}
			}
			if !assert.Len(t, points, 1) {
				return
			}
			assert.Equal(t, uint64(1), points[0].Count)
			attrs := points[0].Attributes
			value, ok := attrs.Value("http.request.canceled")
			assert.True(t, ok, "client metrics must distinguish cancellations from untyped transport failures")
			assert.Equal(t, tc.canceled, value.AsBool())
			value, _ = attrs.Value("test.inherited")
			assert.Equal(t, "kept", value.AsString())
			value, _ = attrs.Value("test.option")
			assert.Equal(t, "kept", value.AsString())
			value, ok = attrs.Value("http.response.status_code")
			assert.Equal(t, tc.status != 0, ok)
			if ok {
				assert.Equal(t, tc.status, value.AsInt64())
			}
			value, ok = attrs.Value("error.type")
			assert.Equal(t, tc.status == 0, ok)
			if tc.name == "untyped transport error" {
				assert.Equal(t, "errors.errorString", value.AsString())
			}
		})
	}
}
