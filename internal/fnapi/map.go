// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnapi

import (
	"context"
	"encoding/json"

	"namespacelabs.dev/foundation/workspace/tasks"
)

type MapRequest struct {
	UserAuth UserAuth `json:"userAuth"`
	FQDN     string   `json:"fqdn"`
	Target   string   `json:"target"`
}

type MapResponse struct {
	FQDN string `json:"fqdn"`
}

func Map(ctx context.Context, fqdn, target string) error {
	return tasks.Action("dns.map-name").Arg("fqdn", fqdn).Arg("target", target).Run(ctx, func(ctx context.Context) error {
		return doMap(ctx, fqdn, target)
	})
}

func doMap(ctx context.Context, fqdn, target string) error {
	userAuth, err := LoadUser()
	if err != nil {
		return err
	}

	req := MapRequest{
		UserAuth: *userAuth,
		FQDN:     fqdn,
		Target:   target,
	}

	return callProdAPI(ctx, "nsl.naming.NamingService/Map", req, func(dec *json.Decoder) error {
		var nr MapResponse
		if err := dec.Decode(&nr); err != nil {
			return err
		}

		return nil
	})
}