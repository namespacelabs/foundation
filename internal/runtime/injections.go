// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package runtime

import (
	"namespacelabs.dev/foundation/std/execution"
)

// ClusterInjection is used in execution.Execute to provide access to the cluster instance.
var ClusterInjection = execution.Define[Cluster]("ns.runtime.cluster")
var ClusterNamespaceInjection = execution.Define[ClusterNamespace]("ns.runtime.cluster-namespace")

func InjectCluster(ns ClusterNamespace) []execution.InjectionInstance {
	return []execution.InjectionInstance{ClusterInjection.With(ns.Cluster()), ClusterNamespaceInjection.With(ns)}
}
