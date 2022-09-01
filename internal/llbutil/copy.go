// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package llbutil

import "github.com/moby/buildkit/client/llb"

type copyOpt func(*llb.CopyInfo)

func FollowSymlink() func(ci *llb.CopyInfo) {
	return func(ci *llb.CopyInfo) {
		ci.FollowSymlinks = true
	}
}

func CopyFrom(src llb.State, srcPath, destPath string, copyInfo ...copyOpt) llb.StateOption {
	return func(s llb.State) llb.State {
		return copy(src, srcPath, s, destPath, copyInfo...)
	}
}

func copy(src llb.State, srcPath string, dest llb.State, destPath string, opts ...copyOpt) llb.State {
	copyInfo := &llb.CopyInfo{
		AllowWildcard:  true,
		AttemptUnpack:  true,
		CreateDestPath: true,
	}

	for _, opt := range opts {
		opt(copyInfo)
	}

	return dest.File(llb.Copy(src, srcPath, destPath, copyInfo), llb.WithCustomNamef("COPY src:%s --> dst:%s", srcPath, destPath))
}
