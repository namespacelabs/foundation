// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package grpc

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/std/go/rpcerrors"
	"namespacelabs.dev/foundation/std/types"
)

var ServerCert *types.Certificate

func Prepare(ctx context.Context, deps ExtensionDeps) error {
	chain := &types.CertificateChain{}

	if err := json.Unmarshal(deps.TlsCert.MustValue(), chain); err != nil {
		return rpcerrors.Errorf(codes.Internal, "failed to unwrap tls cert")
	}

	ServerCert = chain.Server
	return nil
}
