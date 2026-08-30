// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package llbutil

import (
	"fmt"
	"path/filepath"

	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

var GitCredentialsBuildkitSecret string

type RunGo struct {
	Base       llb.State
	SrcMount   llb.State
	WorkingDir string
	Platform   *specs.Platform
}

func (r RunGo) withSrc(src llb.State, ro ...llb.RunOption) llb.ExecState {
	es := r.Base.
		Dir(filepath.Join("/src", r.WorkingDir))
	if r.Platform != nil {
		es = es.AddEnv("GOOS", r.Platform.OS).AddEnv("GOARCH", r.Platform.Architecture)
	}

	if GitCredentialsBuildkitSecret != "" {
		ro = append(ro, llb.AddSecret("/root/.git-credentials", llb.SecretID(GitCredentialsBuildkitSecret)))
	}

	run := es.Run(ro...)
	run.AddMount("/src", src, llb.Readonly)
	run.AddMount("/go/pkg/mod", llb.Scratch(), llb.AsPersistentCacheDir("go-pkg-mod", llb.CacheMountShared))
	run.AddMount("/root/.cache/go-build", llb.Scratch(), llb.AsPersistentCacheDir("go-build", llb.CacheMountShared))
	return run
}

func (r RunGo) With(ro ...llb.RunOption) llb.ExecState {
	return r.withSrc(r.SrcMount, ro...)
}

func (r RunGo) PrepareGoMod(ro ...llb.RunOption) llb.ExecState {
	goMod := filepath.Join(r.WorkingDir, "go.mod")
	goSum := filepath.Join(r.WorkingDir, "go.sum")
	prepared := llb.Scratch().With(
		copyFrom(r.SrcMount, goMod, goMod),
		copyFrom(r.SrcMount, goSum, goSum))
	return r.withSrc(prepared, ro...)
}

func copyFrom(src llb.State, srcPath, destPath string) llb.StateOption {
	return func(s llb.State) llb.State {
		return copy(src, srcPath, s, destPath)
	}
}

func PrefixSh(prefix string, platform *specs.Platform, command string, args ...interface{}) []llb.RunOption {
	if platform != nil {
		prefix += fmt.Sprintf(" %s/%s", platform.OS, platform.Architecture)
	}

	return []llb.RunOption{
		llb.Shlexf(command, args...),
		llb.WithCustomNamef("[%s] %s", prefix, fmt.Sprintf(command, args...)),
	}
}
