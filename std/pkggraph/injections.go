// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package pkggraph

import "namespacelabs.dev/foundation/std/execution"

var (
	PackageLoaderInjection = execution.Define[PackageLoader]("ns.pkggraph.package-loader")
	MutableModuleInjection = execution.Define[MutableModule]("ns.pkggraph.mutable-module")
)
