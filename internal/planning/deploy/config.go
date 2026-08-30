// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"namespacelabs.dev/foundation/framework/deploy"
	"namespacelabs.dev/foundation/std/cfg"
)

var deploymentConfigType = cfg.DefineConfigType[*deploy.Deployment]()

func RequireReason(cfg cfg.Configuration) bool {
	if conf, ok := deploymentConfigType.CheckGet(cfg); ok {
		return conf.GetRequireReason()
	}

	return false
}

func GetConfig(cfg cfg.Configuration) (*deploy.Deployment, bool) {
	return deploymentConfigType.CheckGet(cfg)
}
