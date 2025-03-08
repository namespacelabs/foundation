package tryhard

import (
	"context"
	"errors"
	"net"
	"syscall"
	"time"

	"github.com/cenkalti/backoff"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func CallSideEffectFree0(ctx context.Context, retryable bool, method func(context.Context) error) error {
	_, err := CallSideEffectFree1(ctx, retryable, func(ctx context.Context) (any, error) {
		return nil, method(ctx)
	})

	return err
}

func CallSideEffectFree1[T any](ctx context.Context, retryable bool, method func(context.Context) (T, error)) (T, error) {
	if !retryable {
		return method(ctx)
	}

	b := &backoff.ExponentialBackOff{
		InitialInterval:     500 * time.Millisecond,
		RandomizationFactor: 0.5,
		Multiplier:          1.5,
		MaxInterval:         5 * time.Second,
		MaxElapsedTime:      2 * time.Minute,
		Clock:               backoff.SystemClock,
	}

	b.Reset()

	span := trace.SpanFromContext(ctx)

	var finalRet T
	err := backoff.Retry(func() error {
		ret, methodErr := method(ctx)
		if methodErr != nil {
			// grpc's ConnectionError have a Temporary() signature. If we, for example, write to
			// a channel and that channel is gone, then grpc observes a ECONNRESET. And propagates
			// it as a temporary error. It doesn't know though whether it's safe to retry, so it
			// doesn't.
			if temp, ok := methodErr.(interface{ Temporary() bool }); ok && temp.Temporary() {
				span.RecordError(methodErr, trace.WithAttributes(attribute.Bool("grpc.temporary_error", true)))
				return methodErr
			}

			var netErr *net.OpError
			if errors.As(methodErr, &netErr) {
				if errno, ok := netErr.Err.(syscall.Errno); ok && errno == syscall.ECONNRESET {
					return methodErr // Retry
				}
			}

			return backoff.Permanent(methodErr)
		}

		finalRet = ret
		return nil
	}, backoff.WithContext(b, ctx))

	return finalRet, err
}
