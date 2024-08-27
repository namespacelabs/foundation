// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"maps"
	"os"
	"path"

	builderv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/builder/v1beta"
	"github.com/docker/buildx/store"
	"github.com/docker/buildx/util/dockerutil"
	"github.com/docker/cli/cli/command"
	"github.com/pkg/errors"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func setupServerSideBuildxProxy(ctx context.Context, stateDir, builderName string, use, defaultLoad bool, dockerCli *command.DockerCli, platforms []api.BuildPlatform) error {
	// Generate private and public keys
	privKeyPem, pubKeyPem, err := genPrivAndPublicKeysPEM()
	if err != nil {
		return err
	}

	// Ask IAM server to exchange our tenant token with a certificate using this public key
	cliCert, err := fnapi.ExchangeTenantTokenForClientCert(ctx, string(pubKeyPem))
	if err != nil {
		return err
	}

	serverBuilderConfigs := []*builderv1beta.GetBuilderConfigurationResponse{}
	for _, plat := range platforms {
		// Download the builder config for this platform
		resp, err := api.GetBuilderConfiguration(ctx, plat)
		if err != nil {
			return err
		}

		serverBuilderConfigs = append(serverBuilderConfigs, resp)
	}

	// Write key files in ns state directory
	state, err := ensureStateDir(stateDir, buildkitProxyPath)
	if err != nil {
		return err
	}

	privKeyPath := path.Join(state, "private_key.pem")
	if err := writeFileToPath(privKeyPem, privKeyPath); err != nil {
		return err
	}

	clientCertPath := path.Join(state, "client_cert.pem")
	if err := writeFileToPath([]byte(cliCert.ClientCertificatePem), clientCertPath); err != nil {
		return err
	}

	builderConfigs := []builderConfig{}
	for _, bc := range serverBuilderConfigs {
		serverCAPath := path.Join(state, fmt.Sprintf("server_%s_cert.pem", bc.GetShape().GetMachineArch()))
		if err := writeFileToPath([]byte(bc.GetServerCaPem()), serverCAPath); err != nil {
			return err
		}

		builderConfigs = append(builderConfigs, builderConfig{
			serverConfig: bc,
			serverCAPath: serverCAPath,
		})
	}

	// Create buildx builders
	if err := wireRemoteBuildxProxy(dockerCli, builderName, use, defaultLoad, builderConfigs, privKeyPath, clientCertPath); err != nil {
		return err
	}

	return nil
}

type builderConfig struct {
	serverConfig *builderv1beta.GetBuilderConfigurationResponse
	serverCAPath string
}

func genPrivAndPublicKeysPEM() ([]byte, []byte, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	publicKey := &privateKey.PublicKey

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, nil, err
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	privateKeyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, nil, err
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	return privateKeyPEM, publicKeyPEM, nil
}

func writeFileToPath(content []byte, path string) error {
	return os.WriteFile(path, content, 0600)
}

func wireRemoteBuildxProxy(dockerCli *command.DockerCli, name string, use, defaultLoad bool, builderConfigs []builderConfig, privKeyPath, clientCertPath string) error {
	return withStore(dockerCli, func(txn *store.Txn) error {
		ng, err := txn.NodeGroupByName(name)
		if err != nil {
			if !os.IsNotExist(errors.Cause(err)) {
				return err
			}
		}

		const driver = "remote"

		if ng == nil {
			ng = &store.NodeGroup{
				Name:   name,
				Driver: driver,
			}
		}

		driverOpts := map[string]string{
			"cert": clientCertPath,
			"key":  privKeyPath,
		}

		if defaultLoad {
			// Supported starting with v0.14.0
			driverOpts["default-load"] = "true"
		}

		for _, bc := range builderConfigs {
			var platforms []string
			if bc.serverConfig.GetShape().GetMachineArch() == "arm64" {
				platforms = []string{"linux/arm64"}
			} else {
				platforms = []string{"linux/amd64"}
			}

			doCopy := maps.Clone(driverOpts)
			doCopy["cacert"] = bc.serverCAPath

			if err := ng.Update(bc.serverConfig.GetShape().GetMachineArch(), "tcp://"+bc.serverConfig.GetBuildkitEndpoint()+":443", platforms, true, true, nil, "", doCopy); err != nil {
				return err
			}
		}

		if use {
			ep, err := dockerutil.GetCurrentEndpoint(dockerCli)
			if err != nil {
				return err
			}

			if err := txn.SetCurrent(ep, name, false, false); err != nil {
				return err
			}
		}

		if err := txn.Save(ng); err != nil {
			return err
		}

		return nil
	})
}
