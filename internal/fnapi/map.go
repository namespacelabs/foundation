// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnapi

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
)

type MapRequest struct {
	UserAuth *UserAuth `json:"userAuth"`
	FQDN     string    `json:"fqdn"`
	Target   string    `json:"target"`
}

type MapResponse struct {
	FQDN string `json:"fqdn"`
}

func Map(ctx context.Context, fqdn, target string) error {
	return tasks.Action("dns.map-name").Arg("fqdn", fqdn).Arg("target", target).Run(ctx, func(ctx context.Context) error {
		var nr MapResponse
		err := Call[MapRequest]{
			Endpoint: EndpointAddress,
			Method:   "nsl.naming.NamingService/Map",
			PreAuthenticateRequest: func(ua *UserAuth, rt *MapRequest) error {
				rt.UserAuth = ua
				return nil
			},
		}.Do(ctx, MapRequest{
			FQDN:   fqdn,
			Target: target,
		}, DecodeJSONResponse(&nr))
		if err != nil {
			return fnerrors.New("mapping %q to %q failed: %w", fqdn, target, err)
		}

		return nil
	})
}
