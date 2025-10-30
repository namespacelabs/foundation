package mode

import "context"

type AptProvider struct{}

func (g AptProvider) List() []string {
	return []string{
		"apt",
	}
}

func (g AptProvider) Mode(ctx context.Context, mode string) (Result, error) {
	return Result{
		// TODO
	}, nil
}

func (g AptProvider) Detect(ctx context.Context, dir, mode string) (Result, error) {
	return Result{
		// TODO
	}, nil
}
