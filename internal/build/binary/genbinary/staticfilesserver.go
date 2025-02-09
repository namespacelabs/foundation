// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package genbinary

import (
	"context"
	"fmt"
	"io/fs"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnfs/digestfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func StaticFilesServerBuilder(config *schema.ImageBuildPlan_StaticFilesServer) build.Spec {
	return nginxImage{config}
}

type nginxImage struct {
	config *schema.ImageBuildPlan_StaticFilesServer
}

func (i nginxImage) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	// XXX this is not quite right. We always setup a index.html fallback
	// regardless of content. And that's probably over-reaching. The user should
	// let us know which paths require this fallback.
	var defaultConf memfs.FS
	defaultConf.Add("etc/nginx/conf.d/default.conf", []byte(fmt.Sprintf(`server {
		listen %d;
		server_name localhost;

		location / {
			root %s;
			index index.html;
			try_files $uri /index.html;
		}

		error_page 500 502 503 504 /50x.html;
		location = /50x.html {
			root /usr/share/nginx/html;
		}
}`, i.config.Port, i.config.Dir)))
	config := oci.MakeLayer("conf", compute.Precomputed[fs.FS](&defaultConf, digestfs.Digest))

	nginx := oci.ResolveImage("nginx:1.21.5-alpine", *conf.TargetPlatform())

	// Workaround nscloud related pull-to-push authentication challenges.
	localNginx := oci.LocalCopy(nginx)

	return oci.MergeImageLayers(localNginx, oci.MakeImageFromScratch("nginx-configuration", config)), nil
}

func (nginxImage) PlatformIndependent() bool { return false }

func (nginxImage) Description() string { return "staticFileServer" }
