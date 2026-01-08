// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package wsremote

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/moby/patternmatcher"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs/digestfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

type Sink interface {
	Deposit(context.Context, []*wscontents.FileEvent) (bool, error)
}

// Returns a computable which will produce a local snapshot as expected
// but forwards all filesystem events (e.g. changes, removals) to the specified sink.
func ObserveAndPush(absPath string, excludes []string, sink Sink, digestMode bool, extraInputs ...compute.UntypedComputable) compute.Computable[any] {
	return &observePath{absPath: absPath, excludes: excludes, sink: sink, digestMode: digestMode, extraInputs: extraInputs}
}

type observePath struct {
	absPath     string
	excludes    []string
	sink        Sink
	digestMode  bool
	extraInputs []compute.UntypedComputable

	compute.LocalScoped[any]
}

func (op *observePath) Action() *tasks.ActionEvent {
	return tasks.Action("web.contents.observe")
}

func (op *observePath) Inputs() *compute.In {
	in := compute.Inputs().Str("absPath", op.absPath).Indigestible("not cacheable", "true")
	for k, extra := range op.extraInputs {
		in = in.Computable(fmt.Sprintf("extra:%d", k), extra)
	}
	return in
}

func (op *observePath) Compute(ctx context.Context, _ compute.Resolved) (any, error) {
	fmt.Fprintf(console.Debug(ctx), "wsremote: starting w/ snapshotting %q (excludes: %v)\n", op.absPath, op.excludes)

	excludeMatcher, err := patternmatcher.New(op.excludes)
	if err != nil {
		return nil, err
	}

	snapshot, err := wscontents.SnapshotDirectory(ctx, op.absPath, excludeMatcher, op.digestMode)
	if err != nil {
		return nil, err
	}

	return localObserver{absPath: op.absPath, excludeMatcher: excludeMatcher, digestMode: op.digestMode, snapshot: snapshot, sink: op.sink}, nil
}

type localObserver struct {
	absPath        string
	excludeMatcher *patternmatcher.PatternMatcher
	digestMode     bool
	snapshot       *memfs.FS
	sink           Sink
}

func (lo localObserver) Abs() string { return lo.absPath }
func (lo localObserver) FS() fs.FS   { return lo.snapshot }
func (lo localObserver) ComputeDigest(ctx context.Context) (schema.Digest, error) {
	return digestfs.Digest(ctx, lo.snapshot)
}
