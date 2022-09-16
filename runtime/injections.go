// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import "namespacelabs.dev/foundation/internal/engine/ops"

// ClusterInjection is used in ops.Execute to provide access to the cluster instance.
var ClusterInjection = ops.Define[Cluster]("ns.runtime.cluster")
var ClusterNamespaceInjection = ops.Define[ClusterNamespace]("ns.runtime.cluster-namespace")

func InjectCluster(ns ClusterNamespace) []ops.InjectionInstance {
	return []ops.InjectionInstance{ClusterInjection.With(ns.Cluster()), ClusterNamespaceInjection.With(ns)}
}
