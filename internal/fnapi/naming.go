// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnapi

import (
	"context"
	"encoding/json"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type IssueRequest struct {
	UserAuth    UserAuth     `json:"userAuth"`
	NameRequest NameRequest  `json:"nameRequest"`
	Resource    NameResource `json:"previous"`
}

type NameRequest struct {
	FQDN      string `json:"fqdn,omitempty"`
	Subdomain string `json:"subdomain,omitempty"`
	NoTLS     bool   `json:"noTls"`
	Org       string `json:"org,omitempty"`
}

type IssueResponse struct {
	Resource NameResource `json:"resource"`
}

type NameResource struct {
	ID          ResourceID      `json:"id"`
	FQDN        string          `json:"fqdn"`
	Certificate NameCertificate `json:"certificate"`
}

type ResourceID struct {
	Opaque []byte `json:"opaque"`
}

type NameCertificate struct {
	PrivateKey        []byte `json:"privateKey"`
	CertificateBundle []byte `json:"certificateBundle"`
	CertificateURL    string `json:"certificateUrl"`
}

// JSON annotations below are used for the Arg() serialization below.
type AllocateOpts struct {
	FQDN      string `json:"fqdn,omitempty"`
	Subdomain string `json:"subdomain,omitempty"`
	NoTLS     bool   `json:"-"`
	Org       string `json:"org,omitempty"`

	Stored *NameResource `json:"-"`
}

func AllocateName(ctx context.Context, srv *schema.Server, opts AllocateOpts) (nr *NameResource, err error) {
	err = tasks.Action("dns.allocate-name").Scope(schema.PackageName(srv.PackageName)).Arg("opts", opts).Run(ctx, func(ctx context.Context) error {
		var err error
		nr, err = doAllocateName(ctx, srv, opts)
		return err
	})
	return
}

func doAllocateName(ctx context.Context, srv *schema.Server, opts AllocateOpts) (*NameResource, error) {
	userAuth, err := LoadUser()
	if err != nil {
		return nil, err
	}

	req := IssueRequest{
		UserAuth: *userAuth,
		NameRequest: NameRequest{
			FQDN:      opts.FQDN,
			Subdomain: opts.Subdomain,
			NoTLS:     opts.NoTLS,
			Org:       opts.Org,
		},
	}

	if opts.Stored != nil {
		req.Resource = *opts.Stored
	}

	var nr IssueResponse
	if err := callProdAPI(ctx, "nsl.naming.NamingService/Issue", req, func(dec *json.Decoder) error {
		return dec.Decode(&nr)
	}); err != nil {
		return nil, err
	}

	return &nr.Resource, nil
}
