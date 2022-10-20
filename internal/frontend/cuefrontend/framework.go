// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func parseFramework(loc pkggraph.Location, str string) (schema.Framework, error) {
	switch str {
	case "GO", "GO_GRPC":
		return schema.Framework_GO, nil
	case "WEB":
		return schema.Framework_WEB, nil
	case "OPAQUE":
		return schema.Framework_OPAQUE, nil
	}

	return schema.Framework_FRAMEWORK_UNSPECIFIED,
		fnerrors.UserError(loc, "unrecognized framework: %s", str)
}
