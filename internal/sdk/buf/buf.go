// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buf

type GenerateTmpl struct {
	Version string       `json:"version"`
	Plugins []PluginTmpl `json:"plugins"`
}

type PluginTmpl struct {
	Name   string   `json:"name,omitempty"`
	Remote string   `json:"remote,omitempty"`
	Out    string   `json:"out"`
	Opt    []string `json:"opt"`
}
