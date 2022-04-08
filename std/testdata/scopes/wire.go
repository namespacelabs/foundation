package scopes

import "context"

func ProvideScopedData(_ context.Context, _ string, _ *Input, deps ScopedDataDeps) (*ScopedData, error) {
	return &ScopedData{Data: deps.Data}, nil
}
