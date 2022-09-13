// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package pkggraph

import "namespacelabs.dev/foundation/internal/engine/ops"

var (
	PackageLoaderInjection = ops.Define[PackageLoader]("ns.pkggraph.package-loader")
	MutableModuleInjection = ops.Define[MutableModule]("ns.pkggraph.mutable-module")
)
