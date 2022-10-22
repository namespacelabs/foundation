// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package pkggraph

import "namespacelabs.dev/foundation/std/cfg"

type Context interface {
	cfg.Context
	PackageLoader
}

type SealedContext interface {
	cfg.Context
	SealedPackageLoader
}

type ContextWithMutableModule interface {
	Context
	MutableModule
}

type sealedCtx struct {
	cfg.Context
	SealedPackageLoader
}

var _ SealedContext = sealedCtx{}

func MakeSealedContext(env cfg.Context, pr SealedPackageLoader) SealedContext {
	return sealedCtx{Context: env, SealedPackageLoader: pr}
}
