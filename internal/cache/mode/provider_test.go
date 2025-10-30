package mode_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"namespacelabs.dev/foundation/internal/cache/mode"
)

func TestProvidersMode(t *testing.T) {
	modeA := mode.Result{
		Paths: []string{"/path/to/a1", "/path/to/a2"},
	}
	modeB := mode.Result{
		Paths: []string{"/path/to/b1"},
	}
	modeC := mode.Result{
		Paths: []string{"/path/to/c1", "/path/to/c2", "/path/to/c3"},
	}

	providers := mode.Providers{
		&mockProvider{
			Results: map[string]mode.Result{
				"a": modeA,
				"b": modeB,
			},
		},
		&mockProvider{
			Results: map[string]mode.Result{
				"c": modeC,
			},
		},
	}

	t.Run("erroring provider", func(t *testing.T) {
		expected := errors.New("some error")
		providers := mode.Providers{
			&mockProvider{
				Results: map[string]mode.Result{
					"a": modeA,
					"b": modeB,
				},
			},
			&mockProvider{
				Err: expected,
				Results: map[string]mode.Result{
					"c": modeC,
				},
			},
		}

		_, err := providers.Mode(t.Context())
		require.ErrorIs(t, err, expected)
	})

	t.Run("skip empty paths", func(t *testing.T) {
		providers := mode.Providers{
			&mockProvider{
				Results: map[string]mode.Result{
					"x": {Paths: []string{}},
				},
			},
		}

		results, err := providers.Mode(t.Context())
		require.NoError(t, err)
		require.Empty(t, results)
	})

	t.Run("no filter", func(t *testing.T) {
		results, err := providers.Mode(t.Context())
		require.NoError(t, err)
		require.Equal(t, results, map[string]mode.Result{
			"a": modeA,
			"b": modeB,
			"c": modeC,
		})
	})

	t.Run("unknown", func(t *testing.T) {
		_, err := providers.Mode(t.Context(), "x")
		var modeErr mode.UnknownModeError
		require.ErrorAs(t, err, &modeErr)
		require.Equal(t, "x", modeErr.Mode)
	})

	t.Run("single matched", func(t *testing.T) {
		results, err := providers.Mode(t.Context(), "b")
		require.NoError(t, err)
		require.Len(t, results, 1)
		require.Equal(t, results["b"], modeB)
	})

	t.Run("multiple matched", func(t *testing.T) {
		results, err := providers.Mode(t.Context(), "c", "a")
		require.NoError(t, err)
		require.Equal(t, results, map[string]mode.Result{
			"a": modeA,
			"c": modeC,
		})
	})

	t.Run("some matched", func(t *testing.T) {
		_, err := providers.Mode(t.Context(), "a", "x", "c")
		var modeErr mode.UnknownModeError
		require.ErrorAs(t, err, &modeErr)
		require.Equal(t, "x", modeErr.Mode)
	})
}

func TestProvidersDetect(t *testing.T) {
	modeA := mode.Result{
		Paths: []string{"/path/to/a1", "/path/to/a2"},
	}
	modeB := mode.Result{
		Paths: []string{"/path/to/b1"},
	}
	modeC := mode.Result{
		Paths: []string{"/path/to/c1", "/path/to/c2", "/path/to/c3"},
	}

	providers := mode.Providers{
		&mockProvider{
			Results: map[string]mode.Result{
				"a": modeA,
				"b": modeB,
			},
		},
		&mockProvider{
			Results: map[string]mode.Result{
				"c": modeC,
			},
		},
	}

	t.Run("erroring provider", func(t *testing.T) {
		expected := errors.New("some error")
		providers := mode.Providers{
			&mockProvider{
				Results: map[string]mode.Result{
					"a": modeA,
					"b": modeB,
				},
			},
			&mockProvider{
				Err: expected,
				Results: map[string]mode.Result{
					"c": modeC,
				},
			},
		}

		_, err := providers.Detect(t.Context(), "/some/dir")
		require.ErrorIs(t, err, expected)
	})

	t.Run("skip empty paths", func(t *testing.T) {
		providers := mode.Providers{
			&mockProvider{
				Results: map[string]mode.Result{
					"x": {Paths: []string{}},
				},
			},
		}

		results, err := providers.Detect(t.Context(), "/some/dir")
		require.NoError(t, err)
		require.Empty(t, results)
	})

	t.Run("no filter", func(t *testing.T) {
		results, err := providers.Detect(t.Context(), "/some/dir")
		require.NoError(t, err)
		require.Equal(t, results, map[string]mode.Result{
			"a": modeA,
			"b": modeB,
			"c": modeC,
		})
	})

	t.Run("unknown", func(t *testing.T) {
		_, err := providers.Detect(t.Context(), "/some/dir", "x")
		var modeErr mode.UnknownModeError
		require.ErrorAs(t, err, &modeErr)
		require.Equal(t, "x", modeErr.Mode)
	})

	t.Run("single matched", func(t *testing.T) {
		results, err := providers.Detect(t.Context(), "/some/dir", "b")
		require.NoError(t, err)
		require.Len(t, results, 1)
		require.Equal(t, results["b"], modeB)
	})

	t.Run("multiple matched", func(t *testing.T) {
		results, err := providers.Detect(t.Context(), "/some/dir", "c", "a")
		require.NoError(t, err)
		require.Equal(t, results, map[string]mode.Result{
			"a": modeA,
			"c": modeC,
		})
	})

	t.Run("some matched", func(t *testing.T) {
		_, err := providers.Detect(t.Context(), "/some/dir", "a", "x", "c")
		var modeErr mode.UnknownModeError
		require.ErrorAs(t, err, &modeErr)
		require.Equal(t, "x", modeErr.Mode)
	})
}

type mockProvider struct {
	Err     error
	Results map[string]mode.Result
}

func (m *mockProvider) List() []string {
	keys := make([]string, 0, len(m.Results))
	for k := range m.Results {
		keys = append(keys, k)
	}
	return keys
}

func (m *mockProvider) Mode(ctx context.Context, use string) (mode.Result, error) {
	if m.Err != nil {
		return mode.Result{}, m.Err
	}
	return m.Results[use], nil
}

func (m *mockProvider) Detect(ctx context.Context, dir, use string) (mode.Result, error) {
	if m.Err != nil {
		return mode.Result{}, m.Err
	}
	return m.Results[use], nil
}
