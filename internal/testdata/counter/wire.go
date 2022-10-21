// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package counter

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/testdata/counter/data"
)

type Counter struct {
	name string
	data *data.Data
}

func (c *Counter) Increment() {
	c.data.Value = c.data.Value + 1
}

func (c *Counter) Get() int32 {
	return c.data.Value
}

func (c *Counter) GetName() string {
	return c.name
}

func ProvideCounter(_ context.Context, input *Input, deps CounterDeps) (*Counter, error) {
	if input.GetName() == "" {
		return nil, fmt.Errorf("cannot provide a nameless counter")
	}
	return &Counter{name: input.GetName(), data: deps.Data}, nil
}
