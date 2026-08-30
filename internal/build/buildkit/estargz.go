// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

var ForceEstargz bool

func MaybeForceEstargz(src map[string]string) map[string]string {
	if !ForceEstargz {
		return src
	}

	src["compression"] = "estargz"
	src["oci-mediatypes"] = "true"
	src["force-compress"] = "true"
	return src
}
