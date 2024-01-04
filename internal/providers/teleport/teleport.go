// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package teleport

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/gravitational/teleport/api/profile"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"namespacelabs.dev/foundation/internal/build/registry"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/teleport/configuration"
)

const (
	tshBin          = "tsh"
	appLoginTTLMins = "720" // 12h
)

var (
	loginMinValidityTTL = time.Minute * 10
	teleportConfigType  = cfg.DefineConfigType[*configuration.TeleportProxy]()
)

func Register() {
	client.RegisterConfigurationProvider("teleport", provideCluster)

	cfg.RegisterConfigurationProvider(func(ctx context.Context, conf *configuration.Configuration) ([]proto.Message, error) {
		if conf.GetProxy() == nil {
			return nil, fnerrors.BadInputError("teleport proxy must be specified")
		}

		if conf.Registry == "" {
			return nil, fnerrors.BadInputError("registry must be specified")
		}

		profile, err := profile.FromDir("", conf.GetProxy().GetProfile())
		if err != nil {
			return nil, fnerrors.UsageError("Login with 'tsh login'", "Teleport profile is not found.")
		}

		cert, err := parseCertificate(ctx, profile.TLSCertPath())
		if err != nil {
			return nil, fnerrors.InternalError("failed to load user's certificate")
		}

		if time.Until(cert.NotAfter) < loginMinValidityTTL {
			usage := fmt.Sprintf(
				"Login with 'tsh login --proxy=%s --user=%s %s'",
				profile.WebProxyAddr, profile.Username, profile.SiteName,
			)
			return nil, fnerrors.UsageError(usage, "Teleport credentials have expired.")
		}

		tpRegistryApp := conf.GetProxy().GetRegistryApp()
		if err := tshAppsLogin(ctx, profile, tpRegistryApp); err != nil {
			return nil, fnerrors.InvocationError("tsh", "failed to login to app %q", tpRegistryApp)
		}

		messages := []proto.Message{
			&client.HostEnv{Provider: "teleport"},
			&registry.Registry{
				Url: conf.Registry,
				Transport: &registry.RegistryTransport{
					Tls: &registry.RegistryTransport_TLS{
						Endpoint: fmt.Sprintf("%s.%s", tpRegistryApp, profile.WebProxyAddr),
						Cert:     profile.AppCertPath(tpRegistryApp),
						Key:      profile.UserKeyPath(),
					},
				},
			},
			conf.Proxy,
		}

		return messages, nil
	})
}

func provideCluster(ctx context.Context, cfg cfg.Configuration) (client.ClusterConfiguration, error) {
	conf, ok := teleportConfigType.CheckGet(cfg)
	if !ok {
		return client.ClusterConfiguration{}, fnerrors.InternalError("missing configuration")
	}

	profile, err := profile.FromDir("", conf.GetProfile())
	if err != nil {
		return client.ClusterConfiguration{}, fnerrors.InternalError("failed to resolve teleport profile")
	}

	tshBinPath, err := exec.LookPath(tshBin)
	if err != nil {
		return client.ClusterConfiguration{}, fnerrors.InternalError("missing tsh binary")
	}

	caData, err := os.ReadFile(profile.TLSCAPathCluster(profile.SiteName))
	if err != nil {
		return client.ClusterConfiguration{}, fnerrors.InternalError("failed to reat root CA")
	}

	clusterName := conf.GetKubeCluster()
	contextName := fmt.Sprintf("%s-%s", profile.SiteName, clusterName)
	return client.ClusterConfiguration{
		Config: clientcmdapi.Config{
			Clusters: map[string]*clientcmdapi.Cluster{
				clusterName: {
					Server:                   fmt.Sprintf("https://%s", profile.KubeProxyAddr),
					TLSServerName:            "kube-teleport-proxy-alpn." + profile.SiteName,
					CertificateAuthorityData: []byte(caData),
				},
			},
			Contexts: map[string]*clientcmdapi.Context{
				contextName: {
					Cluster: clusterName,
					Extensions: map[string]runtime.Object{
						"teleport.kube.name": &runtime.Unknown{
							// We need to wrap the kubeName in quotes to make sure it is parsed as a string.
							Raw: []byte(fmt.Sprintf("%q", clusterName)),
						},
					},
					AuthInfo: contextName,
				},
			},
			CurrentContext: contextName,
			AuthInfos: map[string]*clientcmdapi.AuthInfo{
				contextName: {
					Exec: &clientcmdapi.ExecConfig{
						APIVersion: "client.authentication.k8s.io/v1beta1",
						Command:    tshBinPath,
						Args: []string{
							"kube", "credentials", "--kube-cluster", conf.GetKubeCluster(),
							"--teleport-cluster", profile.SiteName, "--proxy", profile.KubeProxyAddr,
						},
						Env:             []clientcmdapi.ExecEnvVar{},
						InteractiveMode: clientcmdapi.NeverExecInteractiveMode,
					},
				},
			},
		},
	}, nil
}

func tshAppsLogin(ctx context.Context, teleportProfile *profile.Profile, app string) error {
	// First we check if there is already a valid certificate as `tsh apps login` is very slow (>3s).
	if err := tasks.Return0(ctx, tasks.Action("teleport.apps.cert").Arg("app", app), func(ctx context.Context) error {
		cert, err := parseCertificate(ctx, teleportProfile.AppCertPath(app))
		if err != nil {
			return err
		}

		// If certificate is not valid after 10m then relogin.
		if time.Until(cert.NotAfter) < loginMinValidityTTL {
			return errors.New("app certificate expires soon")
		}

		return nil
	}); err == nil {
		return nil
	}

	return tasks.Return0(ctx, tasks.Action("tsh.apps.login").Arg("app", app), func(ctx context.Context) error {
		tshBinPath, err := exec.LookPath(tshBin)
		if err != nil {
			return fnerrors.InternalError("missing tsh binary")
		}

		c := exec.CommandContext(ctx, tshBinPath, "apps", "login", app, "--ttl", appLoginTTLMins)
		if err := c.Run(); err != nil {
			return err
		}

		return nil
	})
}

func parseCertificate(ctx context.Context, certPath string) (*x509.Certificate, error) {
	b, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	certData, _ := pem.Decode(b)
	if certData == nil {
		return nil, errors.New("not pem formatted certificate")
	}

	return x509.ParseCertificate(certData.Bytes)
}
