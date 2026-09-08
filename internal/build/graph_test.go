// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package build

import "testing"

type testGraphPass struct {
	finalized int
}

func (p *testGraphPass) Finalize() error {
	p.finalized++
	return nil
}

func TestGraphFinalizesRegisteredPasses(t *testing.T) {
	graph := NewGraph()
	want := &testGraphPass{}
	first, err := graph.Pass("test", func() GraphPass { return want })
	if err != nil {
		t.Fatal(err)
	}
	second, err := graph.Pass("test", func() GraphPass { return &testGraphPass{} })
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("same pass name produced different graph passes")
	}
	if err := graph.Finalize(); err != nil {
		t.Fatal(err)
	}
	if want.finalized != 1 {
		t.Fatalf("finalized = %d, want 1", want.finalized)
	}
	if _, err := graph.Pass("late", func() GraphPass { return &testGraphPass{} }); err == nil {
		t.Fatal("expected registering a pass after finalization to fail")
	}
}
