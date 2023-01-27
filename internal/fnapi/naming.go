// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"

	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/certificates"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

var NamingForceStored = false

type IssueRequest struct {
	UserAuth    *auth.UserAuth `json:"user_auth"`
	NameRequest NameRequest    `json:"name_request"`
	Resource    NameResource   `json:"previous"`
}

type NameRequest struct {
	FQDN  string `json:"fqdn,omitempty"`
	NoTLS bool   `json:"noTls"`
	Org   string `json:"org,omitempty"`
}

type IssueResponse struct {
	Resource NameResource `json:"resource"`
}

type NameResource struct {
	ID          ResourceID      `json:"id"`
	Certificate NameCertificate `json:"certificate"`
}

type ResourceID struct {
	Opaque []byte `json:"opaque"`
}

type NameCertificate struct {
	PrivateKey        []byte `json:"private_key"`
	CertificateBundle []byte `json:"certificate_bundle"`
	CertificateURL    string `json:"certificate_url"`
}

// JSON annotations below are used for the Arg() serialization below.
type AllocateOpts struct {
	Scope     schema.PackageName `json:"-"`
	FQDN      string             `json:"fqdn,omitempty"`
	Subdomain string             `json:"subdomain,omitempty"`
	NoTLS     bool               `json:"-"`
	Org       string             `json:"org,omitempty"`

	Stored *NameResource `json:"-"`
}

func AllocateName(ctx context.Context, opts AllocateOpts) (*NameResource, error) {
	action := tasks.Action("dns.allocate-name")
	if opts.Scope != "" {
		action = action.Scope(opts.Scope)
	}

	return tasks.Return(ctx, action.Arg("opts", opts), func(ctx context.Context) (*NameResource, error) {
		if NamingForceStored && opts.Stored != nil {
			tasks.Attachments(ctx).AddResult("force_stored", true)
			return opts.Stored, nil
		}

		req := IssueRequest{
			NameRequest: NameRequest{
				FQDN:  opts.FQDN,
				NoTLS: opts.NoTLS,
				Org:   opts.Org,
			},
		}

		if opts.Stored != nil {
			req.Resource = *opts.Stored
		}

		var nr IssueResponse

		if err := (Call[IssueRequest]{
			Endpoint: EndpointAddress,
			Method:   "nsl.naming.NamingService/Issue",
			PreAuthenticateRequest: func(ua *auth.UserAuth, rt *IssueRequest) error {
				rt.UserAuth = ua
				return nil
			},
			FetchToken: auth.GenerateToken,
		}).Do(ctx, req, DecodeJSONResponse(&nr)); err != nil {
			return nil, err
		}

		res := &nr.Resource

		if len(res.Certificate.CertificateBundle) > 0 {
			tasks.Attachments(ctx).Attach(tasks.Output("certificate.pem", "application/x-pem-file"), res.Certificate.CertificateBundle)

			if _, ts, err := certificates.CertIsValid(res.Certificate.CertificateBundle); err == nil {
				tasks.Attachments(ctx).AddResult("notAfter", ts)
			}
		}

		return res, nil
	})
}
