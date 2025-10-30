package mode

import "context"

type GoProvider struct{}

func (g GoProvider) List() []string {
	return []string{
		"go",
		"golangci-lint",
	}
}

func (g GoProvider) Mode(ctx context.Context, mode string) (Result, error) {
	return Result{
		Paths: []string{
			"/some/dir",
		},
	}, nil
}

func (g GoProvider) Detect(ctx context.Context, dir, mode string) (Result, error) {
	return Result{
		Paths: []string{
			"/some/dir",
		},
	}, nil
}
