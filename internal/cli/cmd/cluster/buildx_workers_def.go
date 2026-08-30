// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

const staticWorkerDef = `
{
	"record": [
		{
			"ID": "namespace-amd64-builder",
			"Labels": {
				"org.mobyproject.buildkit.worker.executor": "oci",
				"org.mobyproject.buildkit.worker.hostname": "namespace-amd64-builder",
				"org.mobyproject.buildkit.worker.network": "cni",
				"org.mobyproject.buildkit.worker.oci.process-mode": "sandbox",
				"org.mobyproject.buildkit.worker.selinux.enabled": "false",
				"org.mobyproject.buildkit.worker.snapshotter": "overlayfs"
			},
			"platforms": [
				{ "Architecture": "amd64", "OS": "linux" },
				{ "Architecture": "amd64", "OS": "linux", "Variant": "v2" },
				{ "Architecture": "amd64", "OS": "linux", "Variant": "v3" },
				{ "Architecture": "amd64", "OS": "linux", "Variant": "v4" },
				{ "Architecture": "386", "OS": "linux" }
			],
			"GCPolicy": [
				{ "keepDuration": 604800000000000, "keepBytes": 32000000000 },
				{ "keepBytes": 47000000000 },
				{ "all": true, "keepBytes": 50000000000 }
			],
			"BuildkitVersion": {
				"package": "github.com/moby/buildkit",
				"version": "v0.12.0-namespace",
				"revision": "latest"
			}
		},
		{
			"ID": "namespace-arm64-builder",
			"Labels": {
				"org.mobyproject.buildkit.worker.executor": "oci",
				"org.mobyproject.buildkit.worker.hostname": "namespace-arm64-builder",
				"org.mobyproject.buildkit.worker.network": "cni",
				"org.mobyproject.buildkit.worker.oci.process-mode": "sandbox",
				"org.mobyproject.buildkit.worker.selinux.enabled": "false",
				"org.mobyproject.buildkit.worker.snapshotter": "overlayfs"
			},
			"platforms": [
				{ "Architecture": "arm64", "OS": "linux" },
				{ "Architecture": "arm64", "OS": "linux", "Variant": "v6" },
				{ "Architecture": "arm64", "OS": "linux", "Variant": "v7" }
			],
			"GCPolicy": [
				{ "keepDuration": 604800000000000, "keepBytes": 32000000000 },
				{ "keepBytes": 47000000000 },
				{ "all": true, "keepBytes": 50000000000 }
			],
			"BuildkitVersion": {
				"package": "github.com/moby/buildkit",
				"version": "v0.12.0-namespace",
				"revision": "latest"
			}
		}
	]
}
`
