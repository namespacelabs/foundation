// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package compute

import (
	"context"
	"fmt"
	"os"
	"testing"

	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/std/tasks/simplelog"
)

func TestSink(t *testing.T) {
	logLevel := 0
	ctx := tasks.WithSink(context.Background(), simplelog.NewSink(os.Stdout, logLevel))

	ch := make(chan int, 1)
	ch <- 1
	x := &testStream{ch: ch}

	if err := Do(ctx, func(ctx context.Context) error {
		return Continuously(ctx, simpleSinkable{c: &testComputable{intStream: x}, t: t}, nil)
	}); err != nil {
		t.Fatal(err)
	}
}

type simpleSinkable struct {
	c Computable[int]

	t *testing.T
}

func (ts simpleSinkable) Inputs() *In { return Inputs().Computable("c", ts.c) }
func (ts simpleSinkable) Updated(ctx context.Context, deps Resolved) error {
	v := MustGetDepValue(deps, ts.c, "c")

	ts.t.Logf("got value: %v", v)

	const expected = 101
	if v != expected {
		return fmt.Errorf("expected %d, got %d", expected, v)
	}

	return ErrDoneSinking
}
func (ts simpleSinkable) Cleanup(context.Context) error { return nil }

type testComputable struct {
	intStream Computable[hasInt]

	LocalScoped[int]
}

type hasInt interface {
	Int() int
}

func (tc *testComputable) Action() *tasks.ActionEvent { return tasks.Action("testcomputable") }
func (tc *testComputable) Inputs() *In                { return Inputs().Computable("stream", tc.intStream) }
func (tc *testComputable) Output() Output             { return Output{} }
func (tc *testComputable) Compute(_ context.Context, deps Resolved) (int, error) {
	v := MustGetDepValue(deps, tc.intStream, "stream")
	return v.Int() + 100, nil
}

type testStream struct {
	ch chan int

	DoScoped[hasInt]
}

func (tc *testStream) Action() *tasks.ActionEvent { return tasks.Action("teststream") }
func (tc *testStream) Inputs() *In                { return Inputs() }
func (tc *testStream) Output() Output {
	return Output{NonDeterministic: true}
}
func (tc *testStream) Compute(context.Context, Resolved) (hasInt, error) {
	return intWrapper(<-tc.ch), nil
}

type intWrapper int

func (i intWrapper) Int() int { return int(i) }
