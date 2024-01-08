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
	"path/filepath"
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
	tbotBin         = "tbot"
	appLoginTTLMins = "720" // 12h
)

var (
	loginMinValidityTTL = time.Minute * 10
	teleportConfigType  = cfg.DefineConfigType[*configuration.Teleport]()
)

func Register() {
	client.RegisterConfigurationProvider("teleport", provideCluster)

	cfg.RegisterConfigurationProvider(func(ctx context.Context, conf *configuration.Configuration) ([]proto.Message, error) {
		if conf.GetTeleport() == nil {
			return nil, fnerrors.BadInputError("teleport configuration must be specified")
		}

		if conf.Registry == "" {
			return nil, fnerrors.BadInputError("registry must be specified")
		}

		var registryTransport *registry.RegistryTransport
		tpRegistryApp := conf.GetTeleport().GetRegistryApp()

		switch {
		case conf.GetTeleport().GetUserProfile() != "":
			profile, err := profile.FromDir("", conf.GetTeleport().GetUserProfile())
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

			if err := tshAppsLogin(ctx, tpRegistryApp, profile.AppCertPath(tpRegistryApp)); err != nil {
				return nil, fnerrors.InvocationError("tsh", "failed to login to app %q", tpRegistryApp)
			}

			registryTransport = &registry.RegistryTransport{
				Tls: &registry.RegistryTransport_TLS{
					Endpoint: fmt.Sprintf("%s.%s", tpRegistryApp, profile.WebProxyAddr),
					Cert:     profile.AppCertPath(tpRegistryApp),
					Key:      profile.UserKeyPath(),
				},
			}
		case conf.GetTeleport().GetRegistryCertsDir() != "":
			certPath := filepath.Join(conf.GetTeleport().GetRegistryCertsDir(), "tlscert")
			keyPath := filepath.Join(conf.GetTeleport().GetRegistryCertsDir(), "key")
			registryTransport = &registry.RegistryTransport{
				Tls: &registry.RegistryTransport_TLS{
					Endpoint: fmt.Sprintf("%s.%s", tpRegistryApp, conf.GetTeleport().GetProxyUrl()),
					Cert:     certPath,
					Key:      keyPath,
				},
			}
		default:
			return nil, fnerrors.BadInputError("either user_profile or bot_destination_dir must be set")
		}

		messages := []proto.Message{
			&client.HostEnv{Provider: "teleport"},
			&registry.Registry{
				Url:       conf.Registry,
				Transport: registryTransport,
			},
			conf.Teleport,
		}

		return messages, nil
	})
}

func provideCluster(ctx context.Context, cfg cfg.Configuration) (client.ClusterConfiguration, error) {
	conf, ok := teleportConfigType.CheckGet(cfg)
	if !ok {
		return client.ClusterConfiguration{}, fnerrors.InternalError("missing configuration")
	}

	switch {
	case conf.GetUserProfile() != "":
		return teleportUserKubeconfig(ctx, conf)
	case conf.GetKubeCertsDir() != "" && conf.GetRegistryCertsDir() != "":
		return teleportBotKubeconfig(ctx, conf)
	default:
		return client.ClusterConfiguration{}, fnerrors.BadInputError("either user_profile or kube_certs_dir and registry_certs_dir must be set")
	}
}

func teleportUserKubeconfig(ctx context.Context, conf *configuration.Teleport) (client.ClusterConfiguration, error) {
	profile, err := profile.FromDir("", conf.GetUserProfile())
	if err != nil {
		return client.ClusterConfiguration{}, fnerrors.InternalError("failed to resolve teleport profile")
	}

	tshBinPath, err := exec.LookPath(tshBin)
	if err != nil {
		return client.ClusterConfiguration{}, fnerrors.InternalError("missing tsh binary")
	}

	caData, err := os.ReadFile(profile.TLSCAPathCluster(profile.SiteName))
	if err != nil {
		return client.ClusterConfiguration{}, fnerrors.InternalError("failed to read CA")
	}

	kubeClusterName := conf.GetKubeCluster()
	contextName := fmt.Sprintf("%s-%s", profile.SiteName, kubeClusterName)
	return client.ClusterConfiguration{
		Config: clientcmdapi.Config{
			Clusters: map[string]*clientcmdapi.Cluster{
				kubeClusterName: {
					Server:                   fmt.Sprintf("https://%s", profile.KubeProxyAddr),
					TLSServerName:            "kube-teleport-proxy-alpn." + profile.SiteName,
					CertificateAuthorityData: caData,
				},
			},
			Contexts: map[string]*clientcmdapi.Context{
				contextName: {
					Cluster: kubeClusterName,
					Extensions: map[string]runtime.Object{
						// We need to wrap the kubeName in quotes to make sure it is parsed as a string.
						"teleport.kube.name": &runtime.Unknown{Raw: []byte(fmt.Sprintf("%q", kubeClusterName))},
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
						InteractiveMode: clientcmdapi.NeverExecInteractiveMode,
					},
				},
			},
		},
	}, nil
}

func teleportBotKubeconfig(ctx context.Context, conf *configuration.Teleport) (client.ClusterConfiguration, error) {
	if conf.GetProxyUrl() == "" {
		return client.ClusterConfiguration{}, fnerrors.InternalError("missing teleport proxy_url configuration")
	}

	tbotBinPath, err := exec.LookPath(tbotBin)
	if err != nil {
		return client.ClusterConfiguration{}, fnerrors.InternalError("missing tbot binary")
	}

	caFile := fmt.Sprintf("%s/teleport-host-ca.crt", conf.GetKubeCertsDir())
	caData, err := os.ReadFile(caFile)
	if err != nil {
		return client.ClusterConfiguration{}, fnerrors.InternalError("failed to read CA")
	}

	ca, err := parseCertificate(ctx, caFile)
	if err != nil {
		return client.ClusterConfiguration{}, fnerrors.InternalError("failed to parse CA")
	}

	teleportCluster := ca.Subject.CommonName
	kubeClusterName := conf.GetKubeCluster()
	contextName := fmt.Sprintf("%s-%s", teleportCluster, kubeClusterName)
	return client.ClusterConfiguration{
		Config: clientcmdapi.Config{
			Clusters: map[string]*clientcmdapi.Cluster{
				kubeClusterName: {
					Server:                   fmt.Sprintf("https://%s", conf.GetProxyUrl()),
					TLSServerName:            "kube-teleport-proxy-alpn." + teleportCluster,
					CertificateAuthorityData: caData,
				},
			},
			Contexts: map[string]*clientcmdapi.Context{
				contextName: {
					Cluster: kubeClusterName,
					Extensions: map[string]runtime.Object{
						// We need to wrap the kubeName in quotes to make sure it is parsed as a string.
						"teleport.kube.name": &runtime.Unknown{Raw: []byte(fmt.Sprintf("%q", kubeClusterName))},
					},
					AuthInfo: contextName,
				},
			},
			CurrentContext: contextName,
			AuthInfos: map[string]*clientcmdapi.AuthInfo{
				contextName: {
					Exec: &clientcmdapi.ExecConfig{
						APIVersion: "client.authentication.k8s.io/v1beta1",
						Command:    tbotBinPath,
						Args: []string{
							"kube", "credentials",
							"--destination-dir", conf.GetKubeCertsDir(),
						},
						InteractiveMode: clientcmdapi.NeverExecInteractiveMode,
					},
				},
			},
		},
	}, nil
}

func tshAppsLogin(ctx context.Context, app, appCertPath string) error {
	// First we check if there is already a valid certificate as `tbot apps login` is very slow (>3s).
	if err := tasks.Return0(ctx, tasks.Action("teleport.validate.cert").Arg("certificate", appCertPath), func(ctx context.Context) error {
		cert, err := parseCertificate(ctx, appCertPath)
		if err != nil {
			return err
		}

		// If certificate is not valid after 10m then relogin.
		if time.Until(cert.NotAfter) < loginMinValidityTTL {
			return errors.New("certificate expires soon")
		}

		return nil
	}); err == nil {
		return nil
	}

	return tasks.Return0(ctx, tasks.Action("tsh.apps.login").Arg("app", app), func(ctx context.Context) error {
		c := exec.CommandContext(ctx, tshBin, "apps", "login", app, "--ttl", appLoginTTLMins)
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
