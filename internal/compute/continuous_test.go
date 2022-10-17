// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compute

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

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

func TestVersionedSink(t *testing.T) {
	logLevel := 0
	ctx := tasks.WithSink(context.Background(), simplelog.NewSink(os.Stdout, logLevel))

	ch := make(chan int, 1)
	x := &versionedStream{ch: ch}

	ch <- 1

	if err := Do(ctx, func(ctx context.Context) error {
		return Continuously(ctx, &streamSinkable{
			c:        &testComputable{intStream: x},
			t:        t,
			expected: 101,
			onResult: func(got int) (int, bool) {
				if got == 110 {
					return 0, false
				}

				ch <- got - 100 + 1

				return got + 1, true
			},
		}, nil)
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

type streamSinkable struct {
	c Computable[int]

	t        *testing.T
	expected int
	onResult func(int) (int, bool)
}

func (ts *streamSinkable) Inputs() *In { return Inputs().Computable("c", ts.c) }
func (ts *streamSinkable) Updated(ctx context.Context, deps Resolved) error {
	ts.t.Logf("started Updated w/ expected=%d", ts.expected)
	defer ts.t.Log("left Updated")

	vint := MustGetDepValue(deps, ts.c, "c")

	ts.t.Logf("got value: %v", vint)

	if vint != ts.expected {
		return fmt.Errorf("expected %d, got %d", ts.expected, vint)
	}

	expected, shouldContinue := ts.onResult(vint)
	if !shouldContinue {
		return ErrDoneSinking
	}

	ts.expected = expected
	ts.t.Logf("reset expected=%d", expected)

	return nil
}
func (ts *streamSinkable) Cleanup(context.Context) error { return nil }

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

type versionedStream struct {
	ch chan int
	DoScoped[hasInt]
}

func (tc *versionedStream) Action() *tasks.ActionEvent { return tasks.Action("teststream") }
func (tc *versionedStream) Inputs() *In                { return Inputs() }
func (tc *versionedStream) Output() Output {
	return Output{NonDeterministic: true}
}
func (tc *versionedStream) Compute(context.Context, Resolved) (hasInt, error) {
	return testValue{ch: tc.ch, v: <-tc.ch}, nil
}

type testValue struct {
	ch chan int

	v int
}

var _ Versioned = testValue{}

func (av testValue) Int() int { return av.v }

func (av testValue) Observe(ctx context.Context, f func(ResultWithTimestamp[any], bool)) (func(), error) {
	cancel := make(chan struct{})

	go func() {
		for {
			select {
			case <-cancel:
				return
			case v := <-av.ch:
				var rwt ResultWithTimestamp[any]
				rwt.Value = testValue{v: v}
				rwt.Completed = time.Now()
				f(rwt, false)
			}
		}
	}()

	return func() {
		close(cancel)
	}, nil
}
