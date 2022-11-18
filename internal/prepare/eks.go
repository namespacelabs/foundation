// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/aws/configuration/eks"
)

func PrepareEksCluster(clusterName string) compute.Computable[*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.eks-cluster-config").HumanReadablef("Prepare the EKS cluster configuration"),
		compute.Inputs().Str("clusterName", clusterName),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) (*schema.DevHost_ConfigureEnvironment, error) {
			fmt.Fprintf(console.Stdout(ctx), "[✓] Configure Namespace to use EKS cluster %q.\n", clusterName)
			return devhost.MakeConfiguration(&eks.Cluster{Name: clusterName})
		})
}
