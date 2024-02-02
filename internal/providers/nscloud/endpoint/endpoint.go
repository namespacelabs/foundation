// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package endpoint

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/fnapi"
)

var (
	rpcEndpointOverride string
	RegionName          string
)

func SetupFlags(prefix string, flags *pflag.FlagSet, hide bool) {
	endpointFlag := fmt.Sprintf("%sendpoint", prefix)
	regionFlag := fmt.Sprintf("%sregion", prefix)

	flags.StringVar(&rpcEndpointOverride, endpointFlag, "", "Where to dial to when reaching nscloud.")
	flags.StringVar(&RegionName, regionFlag, "eu", "Which region to use.")

	if hide {
		_ = flags.MarkHidden(endpointFlag)
		_ = flags.MarkHidden(regionFlag)
	}
}

func ResolveRegionalEndpoint(ctx context.Context, tok fnapi.ResolvedToken) (string, error) {
	if rpcEndpointOverride != "" {
		return rpcEndpointOverride, nil
	}

	if rpcEndpoint := os.Getenv("NSC_ENDPOINT"); rpcEndpoint != "" {
		return rpcEndpoint, nil
	}

	if tok.PrimaryRegion != "" {
		return "https://api." + tok.PrimaryRegion, nil
	}

	rpcEndpoint := fmt.Sprintf("https://api.%s.nscluster.cloud", RegionName)

	return rpcEndpoint, nil
}
