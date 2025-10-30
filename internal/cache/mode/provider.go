package mode

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"golang.org/x/sync/errgroup"
)

func DefaultProviders() Providers {
	return Providers{
		AptProvider{},
		GoProvider{},
	}
}

type UnknownModeError struct {
	Mode string
}

func (e UnknownModeError) Error() string {
	return "unknown cache mode: " + e.Mode
}

// Provider knows how to provide cache paths for specific modes.
type Provider interface {
	List() []string
	Mode(ctx context.Context, mode string) (Result, error)
	Detect(ctx context.Context, dir, mode string) (Result, error)
}

type Result struct {
	Paths []string `json:"paths"`
}

type Providers []Provider

func (ps Providers) List() []string {
	modes := make([]string, 0, len(ps)*2)
	for _, p := range ps {
		modes = append(modes, p.List()...)
	}
	slices.Sort(modes)
	return modes
}

func (ps Providers) Mode(ctx context.Context, filter ...string) (map[string]Result, error) {
	var m sync.Mutex
	results := make(map[string]Result, len(ps)*2)

	eg, ctx := errgroup.WithContext(ctx)
	for _, provider := range ps {
		for _, mode := range provider.List() {
			switch {
			case len(filter) == 0:
			case slices.Contains(filter, mode):
			default:
				continue
			}

			eg.Go(func() error {
				result, err := provider.Mode(ctx, mode)
				if err != nil {
					return fmt.Errorf("looking up `%s` mode: %w", mode, err)
				}
				if len(result.Paths) > 0 {
					m.Lock()
					results[mode] = result
					m.Unlock()
				}
				return nil
			})
		}
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	for _, mode := range filter {
		if _, ok := results[mode]; !ok {
			return nil, UnknownModeError{Mode: mode}
		}
	}

	return results, nil
}

func (ps Providers) Detect(ctx context.Context, dir string, filter ...string) (map[string]Result, error) {
	var m sync.Mutex
	results := make(map[string]Result, len(ps)*2)

	eg, ctx := errgroup.WithContext(ctx)
	for _, provider := range ps {
		for _, mode := range provider.List() {
			switch {
			case len(filter) == 0:
			case slices.Contains(filter, mode):
			default:
				continue
			}

			eg.Go(func() error {
				result, err := provider.Detect(ctx, dir, mode)
				if err != nil {
					return fmt.Errorf("looking up `%s` mode: %w", mode, err)
				}
				if len(result.Paths) > 0 {
					m.Lock()
					results[mode] = result
					m.Unlock()
				}
				return nil
			})
		}
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	for _, mode := range filter {
		if _, ok := results[mode]; !ok {
			return nil, UnknownModeError{Mode: mode}
		}
	}

	return results, nil
}
