package data

import "context"

func ProvideData(_ context.Context, caller string, _ *Input) (*Data, error) {
	return &Data{Caller: []string{caller}}, nil
}
