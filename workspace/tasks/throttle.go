// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync"

	"google.golang.org/protobuf/encoding/prototext"
	"namespacelabs.dev/foundation/workspace/dirs"
)

var (
	//go:embed throttle.textpb
	embeddedConfig embed.FS

	throttler *throttleState
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

func SetupThrottler(debug io.Writer) error {
	conf, err := parseThrottleConfig(debug)
	if err != nil {
		return err
	}

	throttler = newThrottleState(debug, conf.ThrottleConfiguration)
	return nil
}

func parseThrottleConfig(debug io.Writer) (*ThrottleConfigurations, error) {
	if dir, err := dirs.Config(); err == nil {
		if cfg, err := parseThrottleConfigFrom(os.DirFS(dir)); err == nil {
			fmt.Fprintf(debug, "Using user-provided throttle configuration (loaded from %s).\n", dir)
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

func newThrottleState(debug io.Writer, confs []*ThrottleConfiguration) *throttleState {
	fmt.Fprintln(debug, "Setting up action throttler.")

	ts := &throttleState{}
	ts.cond = sync.NewCond(&ts.mu)
	for _, conf := range confs {
		ts.capacity = append(ts.capacity, &throttleCapacity{c: conf, used: map[string]int32{}})
		fmt.Fprintf(debug, "  %+v\n", conf)
	}
	return ts
}

func (ts *throttleState) AcquireLease(ctx context.Context, wellKnown map[WellKnown]string) (func(), error) {
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
				label, ok = wellKnown[WellKnown(cap.c.CountPerLabel)]
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

func (tc *throttleCapacity) matches(labels map[WellKnown]string) bool {
	for key, value := range tc.c.Labels {
		if chk, ok := labels[WellKnown(key)]; !ok || chk != value {
			return false
		}
	}
	return true
}
