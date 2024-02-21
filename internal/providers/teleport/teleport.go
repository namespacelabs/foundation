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
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/gravitational/teleport/api/profile"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/registry"
	"namespacelabs.dev/foundation/internal/certificates"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/universe/teleport/configuration"
)

const (
	tshBin          = "tsh"
	tbotBin         = "tbot"
	appLoginTTLMins = "720" // 12h
)

var (
	teleportConfigType = cfg.DefineConfigType[*configuration.Teleport]()
)

func Register() {
	client.RegisterConfigurationProvider("teleport", provideCluster)

	cfg.RegisterConfigurationProvider(func(conf *configuration.Configuration) ([]proto.Message, error) {
		if conf.GetTeleport() == nil {
			return nil, fnerrors.BadInputError("teleport configuration must be specified")
		}

		if conf.Registry == "" {
			return nil, fnerrors.BadInputError("registry must be specified")
		}

		if conf.GetTeleport().GetEcrCredentialsProxyApp() != "" {
			oci.RegisterDomainKeychain(registryHost(conf.Registry), ecrTeleportKeychain{conf: conf}, oci.Keychain_UseAlways)
		}

		regTransport, err := registryTransport(conf)
		if err != nil {
			return nil, err
		}

		messages := []proto.Message{
			&client.HostEnv{Provider: "teleport"},
			&registry.Registry{
				Url:       conf.Registry,
				Transport: regTransport,
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
		if err := tshEnsureLogin(ctx, conf); err != nil {
			return client.ClusterConfiguration{}, err
		}

		return teleportUserKubeconfig(ctx, conf)
	case conf.GetKubeCertsDir() != "":
		return teleportBotKubeconfig(ctx, conf)
	default:
		return client.ClusterConfiguration{}, fnerrors.BadInputError("either user_profile or kube_certs_dir must be set")
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
							"--add-keys-to-agent", "no",
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

func tshEnsureLogin(ctx context.Context, conf *configuration.Teleport) error {
	profile, err := profile.FromDir("", conf.GetUserProfile())
	if err != nil {
		return fnerrors.InternalError("failed to resolve teleport profile")
	}

	valid, _, err := certificates.CertFileIsValidFor(profile.TLSCertPath(), time.Duration(0))
	if err != nil {
		return fnerrors.InternalError("failed to load user's certificate")
	}
	if !valid {
		return fnerrors.UsageError("Login with 'tsh login'", "Teleport credentials have expired or expire soon.")
	}

	var appsLogin []string
	if app := conf.GetRegistryApp(); app != "" {
		appsLogin = append(appsLogin, app)
	}
	if app := conf.GetEcrCredentialsProxyApp(); app != "" {
		appsLogin = append(appsLogin, app)
	}

	for _, app := range appsLogin {
		if err := tshAppsLogin(ctx, app, profile.AppCertPath(app)); err != nil {
			return fnerrors.InvocationError("tsh", "failed to login to app %q", app)
		}
	}

	return nil
}

func parseCertificate(ctx context.Context, certPath string) (*x509.Certificate, error) {
	b, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	certData, _ := pem.Decode(b)
	if certData == nil {
		return nil, errors.New("not a pem formatted certificate")
	}

	return x509.ParseCertificate(certData.Bytes)
}

func registryHost(registry string) string {
	rHost := registry
	if u, _ := url.Parse(registry); u != nil && u.Host != "" {
		rHost = u.Host
	}

	return rHost
}

func registryTransport(conf *configuration.Configuration) (*registry.RegistryTransport, error) {
	registryApp := conf.GetTeleport().GetRegistryApp()
	// If Teleport Registry App is not configured, then no need for custom transport as we access directly registry.

	appCreds, err := resolveTeleportAppCreds(conf.GetTeleport(), registryApp)
	if err != nil {
		return nil, err
	}

	if appCreds == nil {
		return nil, nil
	}

	return &registry.RegistryTransport{
		Tls: &registry.RegistryTransport_TLS{
			Endpoint: appCreds.endpoint,
			Cert:     appCreds.certFile,
			Key:      appCreds.keyFile,
		},
	}, nil
}
