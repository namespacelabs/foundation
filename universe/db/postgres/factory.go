package postgres

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

type Factory struct {
	res *resources.Parsed
	t   trace.Tracer
}

func ProvideFactory(ctx context.Context, _ *FactoryArgs, deps ExtensionDeps) (Factory, error) {
	res, err := resources.LoadResources()
	if err != nil {
		return Factory{}, err
	}

	tracer, err := tracing.Tracer(Package__sfr1nt, deps.OpenTelemetry)
	if err != nil {
		return Factory{}, err
	}

	return Factory{res, tracer}, nil
}

func (f Factory) Provide(ctx context.Context, ref string) (*DB, error) {
	return ConnectToResource(ctx, f.res, ref, NewDBOptions{
		Tracer: f.t,
	})
}

func (f Factory) ProvideWithCustomErrors(ctx context.Context, ref string, errW func(context.Context, error) error) (*DB, error) {
	return ConnectToResource(ctx, f.res, ref, NewDBOptions{
		Tracer:       f.t,
		ErrorWrapper: errW,
	})
}
