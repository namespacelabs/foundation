// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

func RegisterDeployOps() {
	register_OpInvokeResourceProvider()
	register_OpWaitForProviderResults()
}
