// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	builderv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/builder/v1beta"
	"github.com/docker/buildx/store"
	"github.com/docker/buildx/util/dockerutil"
	"github.com/docker/cli/cli/command"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/natefinch/atomic"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/metadata"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

func PrepareServerSideBuildxProxy(ctx context.Context, stateDir string, platforms []api.BuildPlatform, conf api.BuilderConfiguration) ([]BuilderConfig, error) {
	credPaths, credContents, err := makeClientCertificate(ctx)
	if err != nil {
		return nil, err
	}

	serverBuilderConfigs := []*builderv1beta.GetBuilderConfigurationResponse{}
	for _, plat := range platforms {
		// Download the builder config for this platform
		resp, err := api.GetBuilderConfiguration(ctx, plat, conf)
		if err != nil {
			return nil, err
		}

		serverBuilderConfigs = append(serverBuilderConfigs, resp)
	}

	privKeyPath := credPaths.privKeyPath
	clientCertPath := credPaths.clientCertPaths

	if privKeyPath == "" && clientCertPath == "" {
		privKeyPath = path.Join(stateDir, "client_cert_and_key.pem")
		clientCertPath = privKeyPath

		certAndKeyPems := credContents.clientCertPem + "\n" + credContents.privKeyPem
		if err := writeAtomicFile([]byte(certAndKeyPems), privKeyPath, 0600); err != nil {
			return nil, err
		}
	}

	builderConfigs := []BuilderConfig{}
	for _, bc := range serverBuilderConfigs {
		serverCAPath := path.Join(stateDir, fmt.Sprintf("server_%s_cert.pem", bc.GetShape().GetMachineArch()))
		if err := writeAtomicFile([]byte(bc.GetServerCaPem()), serverCAPath, 0600); err != nil {
			return nil, err
		}

		builderConfigs = append(builderConfigs, BuilderConfig{
			Platform:             bc.GetShape().GetOs() + "/" + bc.GetShape().GetMachineArch(),
			Arch:                 bc.GetShape().GetMachineArch(),
			FullBuildkitEndpoint: bc.GetFullBuildkitEndpoint(),
			ServerCAPath:         serverCAPath,
			ClientKeyPath:        privKeyPath,
			ClientCertPath:       clientCertPath,
		})
	}

	return builderConfigs, nil
}

func DumpListWorkers(ctx context.Context, bc BuilderConfig) (*controlapi.ListWorkersResponse, error) {
	clientConn, err := CreateGrpcClientConn(bc)
	if err != nil {
		return nil, err
	}

	defer clientConn.Close()

	client := controlapi.NewControlClient(clientConn)
	return client.ListWorkers(ctx, &controlapi.ListWorkersRequest{})
}

func TestServerSideBuildxProxyConnectivity(ctx context.Context, bc BuilderConfig) (bool, error) {
	clientConn, err := CreateGrpcClientConn(bc)
	if err != nil {
		return false, err
	}

	defer clientConn.Close()

	streamDesc := &grpc.StreamDesc{
		ServerStreams: true,
		ClientStreams: true,
	}

	clientStream, err := grpc.NewClientStream(ctx, streamDesc, clientConn, "/nsl.buildkit.ConnectivityTest/Ping")
	if err != nil {
		return false, fmt.Errorf("failed to create client stream to '%s': %v", bc.FullBuildkitEndpoint, err)
	}

	empty := &emptypb.Empty{}
	if err := clientStream.SendMsg(empty); err != nil {
		return false, fmt.Errorf("failed to send message to '%s': %v", bc.FullBuildkitEndpoint, err)
	}

	clientStream.CloseSend()

	if err := clientStream.RecvMsg(empty); err != nil {
		if status.Code(err) == codes.Unimplemented {
			// The endpoint does not implement nsl.buildkit.ConnectivityTest, but is reachable as a grpc server.
			return false, nil
		}

		return false, fmt.Errorf("failed to recv message from '%s': %v", bc.FullBuildkitEndpoint, err)
	}

	return true, nil
}

func CreateGrpcClientConn(bc BuilderConfig) (*grpc.ClientConn, error) {
	endpoint := bc.FullBuildkitEndpoint
	// buildkit expects this to be a URL
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("can't parse '%s' as url: %v", endpoint, err)
	}

	if parsed.Scheme != "tcp" {
		return nil, fmt.Errorf("don't support scheme '%s' in '%s' yet", parsed.Scheme, endpoint)
	}

	cfg := &tls.Config{
		ServerName: parsed.Hostname(),
		RootCAs:    x509.NewCertPool(),
	}

	serverCAs, err := os.ReadFile(bc.ServerCAPath)
	if err != nil {
		return nil, fmt.Errorf("can't read server CA file '%s': %v", bc.ServerCAPath, err)
	}

	if ok := cfg.RootCAs.AppendCertsFromPEM(serverCAs); !ok {
		return nil, fmt.Errorf("can't append server CA certs from '%s'", bc.ServerCAPath)
	}

	clientCert, err := tls.LoadX509KeyPair(bc.ClientCertPath, bc.ClientKeyPath)
	if err != nil {
		return nil, fmt.Errorf("can't read certificate/key ('%s'/'%s'): %v", bc.ClientCertPath, bc.ClientKeyPath, err)
	}

	cfg.Certificates = append(cfg.Certificates, clientCert)

	cl, err := grpc.NewClient(parsed.Host, grpc.WithAuthority(parsed.Hostname()), grpc.WithTransportCredentials(credentials.NewTLS(cfg)))
	if err != nil {
		return nil, fmt.Errorf("can't create client to '%s': %v", endpoint, err)
	}

	return cl, nil
}

func setupServerSideBuildxProxy(ctx context.Context, stateDir, builderName string, use, defaultLoad bool, dockerCli *command.DockerCli, platforms []api.BuildPlatform, conf api.BuilderConfiguration) error {
	builderConfigs, err := PrepareServerSideBuildxProxy(ctx, stateDir, platforms, conf)
	if err != nil {
		return err
	}

	// Create buildx builders
	if err := wireRemoteBuildxProxy(dockerCli, builderName, use, defaultLoad, builderConfigs); err != nil {
		return err
	}

	return nil
}

func RefreshSessionClientCert(ctx context.Context) (bool, error) {
	clientCertPath, publicKey, err := getCertPathToRefresh()
	if err != nil {
		return false, errors.Errorf("refresh session client cert: failed to check existing cert: %v", err)
	}

	if clientCertPath == "" {
		// No client cert to refresh
		return false, nil
	}

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return false, errors.Errorf("refresh session client cert: can not marshal public key: %v", err)
	}

	pubKeyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	newCertPem, err := fetchClientCert(ctx, string(pubKeyPem))

	if err != nil {
		return false, fnerrors.Newf("refresh session client cert: could not issue client cert: %w", err)
	}

	if err := atomic.WriteFile(clientCertPath, strings.NewReader(newCertPem)); err != nil {
		return false, fnerrors.Newf("refresh session client cert: can not write new cert: %w", err)
	}

	return true, nil
}

func DeleteSessionClientCert() error {
	state, err := DetermineStateDir("", BuildkitProxyPath)
	if err != nil {
		return err
	}

	clientCertPath := path.Join(state, "client_cert.pem")
	if _, err := os.Stat(clientCertPath); err == nil {
		return os.Remove(clientCertPath)
	}

	return nil
}

func getCertPathToRefresh() (string, any, error) {
	state, err := DetermineStateDir("", BuildkitProxyPath)
	if err != nil {
		return "", nil, err
	}

	clientCertPath := path.Join(state, "client_cert.pem")

	b, err := os.ReadFile(clientCertPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, nil

		}
		return "", nil, errors.Errorf("could not check if %s is a client cert: %v", clientCertPath, err)
	}

	certData, _ := pem.Decode(b)
	if certData == nil {
		return "", nil, errors.Errorf("%s does not contain a PEM encoded certificate", clientCertPath)
	}

	if certData.Type != "CERTIFICATE" {
		return "", nil, errors.Errorf("%s does not contains a %s instead of a CERTIFICATE", clientCertPath, certData.Type)
	}

	cert, err := x509.ParseCertificate(certData.Bytes)
	if err != nil {
		return "", nil, errors.Errorf("can not parse %s as X.059 certificate: %v", clientCertPath, err)
	}

	if !slices.Contains(cert.Subject.OrganizationalUnit, "sessioncert") {
		return "", nil, nil
	}

	return clientCertPath, cert.PublicKey, nil
}

type clientCredPaths struct {
	privKeyPath     string
	clientCertPaths string
}

type clientCredContents struct {
	privKeyPem    string
	clientCertPem string
}

func makeClientCertificate(ctx context.Context) (clientCredPaths, clientCredContents, error) {
	md, err := metadata.InstanceMetadataFromFile()
	if err == nil {
		// This is running in a Namespace instance.
		// -> use prepared instance client certificate
		return clientCredPaths{privKeyPath: md.Certs.PrivateKeyPath, clientCertPaths: md.Certs.PublicPemPath}, clientCredContents{}, nil
	}

	// Not running in a Namespace instance.
	// We generate public and private key and ask IAM service to issue a client certificate.

	privKeyPem, pubKeyPem, err := genPrivAndPublicKeysPEM()
	if err != nil {
		return clientCredPaths{}, clientCredContents{}, err
	}

	certPem, err := fetchClientCert(ctx, string(pubKeyPem))

	if err != nil {
		return clientCredPaths{}, clientCredContents{}, fnerrors.Newf("could not issue client cert: %w", err)
	}

	return clientCredPaths{}, clientCredContents{privKeyPem: string(privKeyPem), clientCertPem: (certPem)}, nil
}

func fetchClientCert(ctx context.Context, pubKeyPem string) (string, error) {
	tok, err := fnapi.FetchToken(ctx)
	if err != nil {
		return "", err
	}

	if tok.IsSessionToken() {
		return tok.ExchangeForSessionClientCert(ctx, string(pubKeyPem), fnapi.IssueSessionClientCertFromSession)
	} else {
		return fnapi.IssueTenantClientCertFromToken(ctx, tok, string(pubKeyPem))
	}
}

type BuilderConfig struct {
	Platform             string // e.g. "linux/amd64"
	Arch                 string // e.g. "amd64"
	FullBuildkitEndpoint string
	ServerCAPath         string
	ClientKeyPath        string
	ClientCertPath       string
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

func writeAtomicFile(content []byte, targetpath string, mode os.FileMode) error {
	// Would want to use atomic.WriteFile, but setting file mode.
	dir, file := filepath.Split(targetpath)
	if dir == "" {
		dir = "."
	}

	if _, err := dirs.Ensure(dir, nil); err != nil {
		return err
	}

	tmpfile, err := os.CreateTemp(dir, file)
	if err != nil {
		return fmt.Errorf("create temp file failed: %v", err)
	}
	defer func() {
		tmpfile.Close()
		_ = os.Remove(tmpfile.Name())
	}()

	if _, err := io.Copy(tmpfile, bytes.NewReader(content)); err != nil {
		return fmt.Errorf("write data to temp file %s failed: %v", tmpfile.Name(), err)
	}
	if err := tmpfile.Sync(); err != nil {
		return fmt.Errorf("flush temp file %s failed: %v", tmpfile.Name(), err)
	}
	if err := tmpfile.Close(); err != nil {
		return fmt.Errorf("close temp file %s failed: %v", tmpfile.Name(), err)
	}

	if err := os.Chmod(tmpfile.Name(), mode); err != nil {
		return fmt.Errorf("chmod temp file %s failed: %v", tmpfile.Name(), err)
	}

	return atomic.ReplaceFile(tmpfile.Name(), targetpath)
}

func wireRemoteBuildxProxy(dockerCli *command.DockerCli, name string, use, defaultLoad bool, builderConfigs []BuilderConfig) error {
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

		for _, bc := range builderConfigs {
			platforms := []string{bc.Platform}

			driverOpts := map[string]string{
				"cert":   bc.ClientCertPath,
				"key":    bc.ClientKeyPath,
				"cacert": bc.ServerCAPath,
			}

			if defaultLoad {
				// Supported starting with v0.14.0
				driverOpts["default-load"] = "true"
			}

			if err := ng.Update(bc.Arch, bc.FullBuildkitEndpoint, platforms, true, true, nil, "", driverOpts); err != nil {
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
