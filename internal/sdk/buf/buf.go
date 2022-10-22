// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buf

type GenerateTmpl struct {
	Version string       `json:"version"`
	Plugins []PluginTmpl `json:"plugins"`
}

type PluginTmpl struct {
	Name string   `json:"name,omitempty"`
	Path string   `json:"path,omitempty"`
	Out  string   `json:"out"`
	Opt  []string `json:"opt"`
}
