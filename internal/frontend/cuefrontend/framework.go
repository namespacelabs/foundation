// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
		fnerrors.NewWithLocation(loc, "unrecognized framework: %s", str)
}
