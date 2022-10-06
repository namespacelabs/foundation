// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package pkggraph

import "namespacelabs.dev/foundation/std/execution"

var (
	PackageLoaderInjection = execution.Define[PackageLoader]("ns.pkggraph.package-loader")
	MutableModuleInjection = execution.Define[MutableModule]("ns.pkggraph.mutable-module")
)
