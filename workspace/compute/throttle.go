// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compute

import (
	"context"
	"embed"
	"io/fs"
	"os"
	"sync"

	"google.golang.org/protobuf/encoding/prototext"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var (
	//go:embed throttle.textpb
	embeddedConfig embed.FS
)

type throttleState struct {
	mu       sync.Mutex
	cond     *sync.Cond
	capacity []*throttleCapacity
}

const marker = "-"

type throttleCapacity struct {
	c    *ThrottleConfiguration
	used map[string]int32 // Total amount of capacity used per value.
}

func parseThrottleConfig() (*ThrottleConfigurations, error) {
	if dir, err := dirs.Config(); err == nil {
		if cfg, err := parseThrottleConfigFrom(os.DirFS(dir)); err == nil {
			return cfg, nil
		}
	}

	return parseThrottleConfigFrom(embeddedConfig)
}

func parseThrottleConfigFrom(fsys fs.FS) (*ThrottleConfigurations, error) {
	bytes, err := fs.ReadFile(fsys, "throttle.textpb")
	if err != nil {
		return nil, err
	}

	confs := &ThrottleConfigurations{}
	if err := prototext.Unmarshal(bytes, confs); err != nil {
		return nil, err
	}

	return confs, nil
}

func newThrottleState(confs []*ThrottleConfiguration) *throttleState {
	ts := &throttleState{}
	ts.cond = sync.NewCond(&ts.mu)
	for _, conf := range confs {
		ts.capacity = append(ts.capacity, &throttleCapacity{c: conf, used: map[string]int32{}})
	}
	return ts
}

func (ts *throttleState) AcquireLease(ctx context.Context, wellKnown map[tasks.WellKnown]string) (func(), error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	for {
		var needsCap bool
		var incs, decs []func()
		for _, cap := range ts.capacity {
			cap := cap // Capture cap.

			if !cap.matches(wellKnown) {
				continue
			}

			var label string
			if cap.c.CountPerLabel != "" {
				var ok bool
				label, ok = wellKnown[tasks.WellKnown(cap.c.CountPerLabel)]
				if !ok {
					continue
				}
			} else {
				label = marker
			}

			if v, ok := cap.used[label]; !ok || v < cap.c.Capacity {
				incs = append(incs, func() {
					cap.used[label]++
				})
				decs = append(decs, func() {
					cap.used[label]--
				})
			} else {
				needsCap = true
			}
		}

		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if !needsCap {
			for _, inc := range incs {
				inc()
			}

			return func() {
				ts.mu.Lock()
				for _, dec := range decs {
					dec()
				}
				ts.cond.Broadcast()
				ts.mu.Unlock()
			}, nil
		}

		ts.cond.Wait()
	}
}

func (tc *throttleCapacity) matches(labels map[tasks.WellKnown]string) bool {
	for key, value := range tc.c.Labels {
		if chk, ok := labels[tasks.WellKnown(key)]; !ok || chk != value {
			return false
		}
	}
	return true
}
