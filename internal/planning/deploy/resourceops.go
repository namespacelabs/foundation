// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

func RegisterDeployOps() {
	register_OpWaitForProviderResults()
	register_OpCaptureServerConfig()
}
