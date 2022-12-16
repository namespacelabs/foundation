// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cloudflare/cloudflared/certutil"
	"github.com/cloudflare/cloudflared/cfapi"
	"github.com/rs/zerolog"
	"namespacelabs.dev/foundation/framework/resources/provider"
	"namespacelabs.dev/foundation/library/cloudflare/tunnel"
	"namespacelabs.dev/foundation/library/cloudflare/tunnel/configuration"
)

func main() {
	_, p := provider.MustPrepare[*tunnel.IngressIntent]()

	if err := apply(p.Intent.Hostname); err != nil {
		log.Fatal(err)
	}

	p.EmitResult(&tunnel.IngressInstance{})
}

func apply(hostnames []string) error {
	tunnelID, err := configuration.ReadTunnelIDFromEnv("CF_TUNNEL_CREDENTIALS")
	if err != nil {
		return err
	}

	zlog := zerolog.New(os.Stdout).With().Timestamp().Logger()

	client, err := makeClient(&zlog)
	if err != nil {
		return err
	}

	for _, h := range hostnames {
		route := cfapi.NewDNSRoute(h, true)
		summary, err := client.RouteTunnel(tunnelID, route)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stdout, "%s: %s\n", h, summary.SuccessSummary())
	}

	return nil
}

func makeClient(log *zerolog.Logger) (*cfapi.RESTClient, error) {
	cert, err := credentials()
	if err != nil {
		return nil, err
	}

	userAgent := fmt.Sprintf("namespace/1.0")
	return cfapi.NewRESTClient(
		"https://api.cloudflare.com/client/v4",
		cert.AccountID,
		cert.ZoneID,
		cert.APIToken,
		userAgent,
		log,
	)
}

func credentials() (*certutil.OriginCert, error) {
	return certutil.DecodeOriginCert([]byte(os.Getenv("CF_TUNNEL_CERT_PEM")))
}
