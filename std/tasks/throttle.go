// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tasks

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync"

	"google.golang.org/protobuf/encoding/prototext"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

type WellKnown string

const (
	WkAction = "action"
)

var (
	BaseDefaultConfig = computeDefaultConfig()

	baseTestConfig = []*ThrottleConfiguration{
		{Labels: map[string]string{"action": "test"}, Capacity: 5},
	}
)

func computeDefaultConfig() []*ThrottleConfiguration {
	confs := []*ThrottleConfiguration{
		{Labels: map[string]string{"action": "lowlevel.invocation"}, Capacity: 3},
		{Labels: map[string]string{"action": "go.build.binary"}, Capacity: 3},
		{Labels: map[string]string{"action": "vcluster.create"}, Capacity: 2},
		{Labels: map[string]string{"action": "vcluster.access"}, Capacity: 2},
	}

	if !environment.IsRunningInCI() {
		// This is for presentation purposes alone; so users don't see a bunch of waiting provision.invoke.
		confs = append(confs, &ThrottleConfiguration{Labels: map[string]string{"action": "provision.invoke"}, Capacity: 3})
	}

	return confs
}

var (
	_throttleKey = contextKey("fn.workspace.tasks.throttler")
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

func LoadThrottlerConfig(ctx context.Context, debugLog io.Writer) *ThrottleConfigurations {
	if dir, err := dirs.Config(); err == nil {
		if cfg, err := parseThrottleConfigFrom(os.DirFS(dir)); err == nil {
			fmt.Fprintf(debugLog, "Using user-provided throttle configuration (loaded from %s).\n", dir)
			return cfg
		}
	}

	configs := &ThrottleConfigurations{}
	configs.ThrottleConfiguration = append(configs.ThrottleConfiguration, BaseDefaultConfig...)
	configs.ThrottleConfiguration = append(configs.ThrottleConfiguration, baseTestConfig...)
	return configs
}

func ContextWithThrottler(ctx context.Context, debugLog io.Writer, confs *ThrottleConfigurations) context.Context {
	return context.WithValue(ctx, _throttleKey, newThrottleState(debugLog, confs.ThrottleConfiguration))
}

func throttlerFromContext(ctx context.Context) *throttleState {
	if s, ok := ctx.Value(_throttleKey).(*throttleState); ok {
		return s
	}
	return nil
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
	if ts == nil {
		return nil, nil
	}

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
