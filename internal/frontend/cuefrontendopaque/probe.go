// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"bytes"
	"encoding/json"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueProbe struct {
	Http *cueHttpProbe `json:"http"`
	Exec *cueExecProbe `json:"exec"`
}

type cueHttpProbe struct {
	Port int    `json:"port"`
	Path string `json:"path"`
}

type cueExecProbe struct {
	Command []string
}

var _ json.Unmarshaler = &cueExecProbe{}

func (e *cueExecProbe) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	tok, err := d.Token()
	if err != nil {
		return err
	}

	if str, ok := tok.(string); ok {
		e.Command = []string{str}
		return nil
	}

	if tok == json.Delim('[') {
		return json.Unmarshal(data, &e.Command)
	}

	return fnerrors.BadInputError("failed to parse exec probe, unexpected token %v", tok)
}

func parseProbes(loc pkggraph.Location, base []*schema.Probe, server cueServer) ([]*schema.Probe, error) {
	if server.ReadinessProbe != nil && server.Probes != nil {
		return nil, fnerrors.AttachLocation(loc, fnerrors.BadInputError("probes and probe are exclusive"))
	}

	probes := base
	if server.ReadinessProbe != nil {
		probe, err := toProbe(loc, runtime.FnServiceReadyz, *server.ReadinessProbe)
		if err != nil {
			return nil, err
		}

		probes = append(probes, probe)
	}

	if server.Probes != nil {
		for name, probe := range server.Probes {
			kind, err := parseProbeKind(name)
			if err != nil {
				return nil, fnerrors.AttachLocation(loc, err)
			}

			probe, err := toProbe(loc, kind, probe)
			if err != nil {
				return nil, err
			}

			probes = append(probes, probe)
		}
	}

	// Check for collisions.
	index := make(map[string]*schema.Probe)

	for _, probe := range probes {
		if existing, ok := index[probe.Kind]; ok {
			if proto.Equal(probe, existing) {
				continue
			}
			return nil, fnerrors.AttachLocation(loc, fnerrors.BadInputError("found conflicting probe definitions for %q (%v vs %v)", probe.Kind, probe, existing))
		} else {
			index[probe.Kind] = probe
		}
	}

	return probes, nil
}

func toProbe(loc pkggraph.Location, kind string, probe cueProbe) (*schema.Probe, error) {
	if probe.Http != nil && probe.Exec != nil {
		return nil, fnerrors.AttachLocation(loc, fnerrors.BadInputError("probes: http and exec are exclusive"))
	}

	if probe.Http != nil {
		return &schema.Probe{
			Kind: kind,
			Http: &schema.Probe_Http{
				Path:          probe.Http.Path,
				ContainerPort: int32(probe.Http.Port),
			},
		}, nil
	}

	if probe.Exec != nil {
		return &schema.Probe{
			Kind: kind,
			Exec: &schema.Probe_Exec{
				Command: probe.Exec.Command,
			},
		}, nil
	}

	return nil, fnerrors.AttachLocation(loc, fnerrors.BadInputError("unknown probe type"))
}

func parseProbeKind(name string) (string, error) {
	switch name {
	case "readiness":
		return runtime.FnServiceReadyz, nil
	case "liveness":
		return runtime.FnServiceLivez, nil
	default:
		return "", fnerrors.BadInputError("%s: unsupported probe kind", name)
	}
}
