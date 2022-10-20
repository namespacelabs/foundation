// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package yarnplugin

import (
	"embed"
	"io/fs"
	"sync"
)

var (
	//go:embed bundles/@yarnpkg/plugin-fn.js
	resources     embed.FS
	pluginContent []byte
	vonce         sync.Once
)

func PluginContent() []byte {
	vonce.Do(func() {
		var err error
		pluginContent, err = fs.ReadFile(resources, "bundles/@yarnpkg/plugin-fn.js")
		if err != nil {
			panic(err)
		}
	})
	return pluginContent
}
