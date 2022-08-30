// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package grpc

import "namespacelabs.dev/foundation/schema"

var ServerCert *schema.Certificate

// chain := &types.CertificateChain{}

// if err := json.Unmarshal(deps.TlsCert.MustValue(), chain); err != nil {
// 	return rpcerrors.Errorf(codes.Internal, "failed to unwrap tls cert")
// }

// ServerCert = chain.Server
