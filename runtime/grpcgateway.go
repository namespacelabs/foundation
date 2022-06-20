// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"namespacelabs.dev/foundation/schema"
)

const GrpcHttpTranscodeNode schema.PackageName = "namespacelabs.dev/foundation/std/grpc/httptranscoding"

const (
	GrpcGatewayServiceName = "grpc-gateway"

	// XXX this is not quite right; it's just a simple mechanism for language and runtime
	// to communicate. Ideally the schema would incorporate a gRPC map.
	kindNeedsGrpcGateway = "needs-grpc-gateway"
)
