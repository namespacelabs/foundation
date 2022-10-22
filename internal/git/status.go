// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package git

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"time"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

// Adapted from Go's src/cmd/go/internal/vcs/vcs.go

// Status is the current state of a local repository.
type Status struct {
	Revision    string    // Optional.
	CommitTime  time.Time // Optional.
	Uncommitted bool      // Required.
}

func FetchStatus(ctx context.Context, rootDir string) (Status, error) {
	out, errOut, err := RunGit(ctx, rootDir, "status", "--porcelain")
	if err != nil {
		if errOut != nil && bytes.Contains(errOut, []byte("not a git repository")) {
			return Status{}, nil
		} else {
			return Status{}, err
		}
	}
	uncommitted := len(out) > 0

	// "git status" works for empty repositories, but "git show" does not.
	// Assume there are no commits in the repo when "git show" fails with
	// uncommitted files and skip tagging revision / committime.
	var rev string
	var commitTime time.Time
	out, errOut, err = RunGit(ctx, rootDir, "show", "-s", "--no-show-signature", "--format=%H:%ct")
	if err != nil && !uncommitted {
		_, _ = console.Stderr(ctx).Write(errOut)
		return Status{}, err
	} else if err == nil {
		rev, commitTime, err = parseRevTime(out)
		if err != nil {
			_, _ = console.Stderr(ctx).Write(errOut)
			return Status{}, err
		}
	}

	return Status{
		Revision:    rev,
		CommitTime:  commitTime,
		Uncommitted: uncommitted,
	}, nil
}

// parseRevTime parses commit details in "revision:seconds" format.
func parseRevTime(out []byte) (string, time.Time, error) {
	buf := string(bytes.TrimSpace(out))

	i := strings.IndexByte(buf, ':')
	if i < 1 {
		return "", time.Time{}, fnerrors.New("unrecognized VCS tool output")
	}
	rev := buf[:i]

	secs, err := strconv.ParseInt(string(buf[i+1:]), 10, 64)
	if err != nil {
		return "", time.Time{}, fnerrors.InternalError("unrecognized VCS tool output: %w", err)
	}

	return rev, time.Unix(secs, 0), nil
}
