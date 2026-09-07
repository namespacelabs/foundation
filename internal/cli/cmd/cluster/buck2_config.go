// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

const (
	buck2IngressAuthHeader = "x-nsc-ingress-auth"
	buck2ConfigFileName    = "50-namespace"
	buck2ConfigDirName     = ".buckconfig.d"
)

type buck2Setup struct {
	EngineAddress      string        `json:"engine_address,omitempty"`
	ActionCacheAddress string        `json:"action_cache_address,omitempty"`
	CasAddress         string        `json:"cas_address,omitempty"`
	TLS                bool          `json:"tls"`
	TLSClientCert      string        `json:"tls_client_cert,omitempty"`
	InstanceName       string        `json:"instance_name,omitempty"`
	StaticToken        string        `json:"static_token,omitempty"`
	TokenDuration      time.Duration `json:"token_duration,omitempty"`
	RemoteExecution    bool          `json:"remote_execution"`
	Path               string        `json:"buckconfig_path,omitempty"`
	clientIdentity     []byte
}

func setBuck2ClientIdentity(out *buck2Setup, certificate, privateKey []byte) error {
	if len(certificate) == 0 || len(privateKey) == 0 {
		return fnerrors.Newf("received incomplete mTLS credentials")
	}

	out.clientIdentity = append(append(append([]byte(nil), certificate...), '\n'), privateKey...)
	return nil
}

func buck2CacheOnlySetup(endpoint, instanceName string) (buck2Setup, error) {
	address, tls, err := buck2Address(endpoint)
	if err != nil {
		return buck2Setup{}, err
	}

	// Buck2 requires engine_address even when no execution platform enables
	// remote execution, so cache-only configuration points it at storage too.
	return buck2Setup{
		EngineAddress:      address,
		ActionCacheAddress: address,
		CasAddress:         address,
		TLS:                tls,
		InstanceName:       instanceName,
	}, nil
}

func buck2ExecutionSetup(schedulerEndpoint, storageEndpoint, instanceName string) (buck2Setup, error) {
	engine, engineTLS, err := buck2Address(schedulerEndpoint)
	if err != nil {
		return buck2Setup{}, err
	}
	storage, storageTLS, err := buck2Address(storageEndpoint)
	if err != nil {
		return buck2Setup{}, err
	}
	if engineTLS != storageTLS {
		return buck2Setup{}, fnerrors.Newf("scheduler (%q) and storage (%q) endpoints disagree on TLS, which Buck2 cannot express", schedulerEndpoint, storageEndpoint)
	}

	return buck2Setup{
		EngineAddress:      engine,
		ActionCacheAddress: storage,
		CasAddress:         storage,
		TLS:                engineTLS,
		InstanceName:       instanceName,
		RemoteExecution:    true,
	}, nil
}

// buck2Address rewrites a Namespace endpoint into Buck2's address and TLS
// fields. Buck2 accepts grpc addresses and configures TLS separately.
func buck2Address(endpoint string) (string, bool, error) {
	if endpoint == "" {
		return "", false, fnerrors.Newf("endpoint is empty")
	}
	if strings.ContainsAny(endpoint, "\r\n") {
		return "", false, fnerrors.Newf("endpoint %q contains a newline", endpoint)
	}

	scheme, host, found := strings.Cut(endpoint, "://")
	if !found {
		return "grpc://" + endpoint, true, nil
	}
	if host == "" {
		return "", false, fnerrors.Newf("endpoint %q has no host", endpoint)
	}

	switch scheme {
	case "grpcs", "https":
		return "grpc://" + host, true, nil
	case "grpc", "http":
		return "grpc://" + host, false, nil
	default:
		return "", false, fnerrors.Newf("cannot translate endpoint %q for Buck2: unsupported scheme %q", endpoint, scheme)
	}
}

var buck2EnvVarRef = regexp.MustCompile(`\$[a-zA-Z_][a-zA-Z_0-9]*`)

func validateBuck2HeaderValue(token string) error {
	if strings.ContainsAny(token, ",\r\n") {
		return fnerrors.InternalError("token contains a comma or newline, which Buck2 cannot carry in http_headers")
	}
	if match := buck2EnvVarRef.FindString(token); match != "" {
		return fnerrors.InternalError("token contains %q, which Buck2 would expand as an environment variable", match)
	}
	return nil
}

func toBuck2Config(out buck2Setup) ([]byte, error) {
	if err := validateBuck2HeaderValue(out.StaticToken); err != nil {
		return nil, err
	}
	if out.StaticToken != "" && out.TLSClientCert != "" {
		return nil, fnerrors.InternalError("Buck2 configuration contains both static and mTLS credentials")
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"instance name", out.InstanceName},
		{"client identity path", out.TLSClientCert},
	} {
		if strings.ContainsAny(field.value, "\r\n") {
			return nil, fnerrors.InternalError("%s contains a newline", field.name)
		}
		if match := buck2EnvVarRef.FindString(field.value); match != "" {
			return nil, fnerrors.InternalError("%s contains %q, which Buck2 would expand as an environment variable", field.name, match)
		}
	}

	var buf bytes.Buffer
	fmt.Fprintln(&buf, "# Generated by nsc; do not edit.")
	fmt.Fprintln(&buf)
	fmt.Fprintln(&buf, "[buck2_re_client]")
	fmt.Fprintf(&buf, "engine_address = %s\n", out.EngineAddress)
	fmt.Fprintf(&buf, "action_cache_address = %s\n", out.ActionCacheAddress)
	fmt.Fprintf(&buf, "cas_address = %s\n", out.CasAddress)
	fmt.Fprintf(&buf, "tls = %t\n", out.TLS)
	if out.TLSClientCert != "" {
		fmt.Fprintf(&buf, "tls_client_cert = %s\n", out.TLSClientCert)
	}
	if out.InstanceName != "" {
		fmt.Fprintf(&buf, "instance_name = %s\n", out.InstanceName)
	}
	if out.StaticToken != "" {
		fmt.Fprintf(&buf, "http_headers = %s:Bearer %s\n", buck2IngressAuthHeader, out.StaticToken)
	}
	return buf.Bytes(), nil
}

func resolveBuck2ConfigPath(configPath string) (string, bool, error) {
	if configPath != "" {
		return configPath, false, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false, fnerrors.Newf("failed to determine the home directory: %w", err)
	}
	return filepath.Join(home, buck2ConfigDirName, buck2ConfigFileName), true, nil
}

func warnUnreadBuck2ConfigPath(ctx context.Context, path string) {
	switch filepath.Base(filepath.Dir(path)) {
	case buck2ConfigDirName, "buckconfig.d":
		return
	}
	switch filepath.Base(path) {
	case ".buckconfig", ".buckconfig.local", "buckconfig":
		return
	}

	fmt.Fprintf(console.Warnings(ctx), "Buck2 does not read configuration from %s.\n", path)
	fmt.Fprintln(console.Warnings(ctx), "Remote execution can only be configured from .buckconfig, .buckconfig.local, a .buckconfig.d/ directory, or /etc/buckconfig.d/.")
}

func emitBuck2Config(ctx context.Context, out buck2Setup, configPath, output, configuration string) error {
	path, isDefault, err := resolveBuck2ConfigPath(configPath)
	if err != nil {
		return err
	}
	if len(out.clientIdentity) > 0 {
		out.TLSClientCert, err = filepath.Abs(path + ".tls.pem")
		if err != nil {
			return fnerrors.Newf("failed to resolve the client identity path: %w", err)
		}
	}

	data, err := toBuck2Config(out)
	if err != nil {
		return err
	}
	if len(out.clientIdentity) > 0 {
		if err := writeCredentialFile(out.TLSClientCert, out.clientIdentity); err != nil {
			return err
		}
	}
	if err := writeCredentialFile(path, data); err != nil {
		return err
	}
	out.Path = path

	switch output {
	case "json":
		encoder := json.NewEncoder(console.Stdout(ctx))
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(out); err != nil {
			return fnerrors.InternalError("failed to encode response as JSON: %w", err)
		}
	case "plain":
		if !isDefault {
			warnUnreadBuck2ConfigPath(ctx, path)
		}
		fmt.Fprintf(console.Stdout(ctx), "Wrote Buck2 configuration for %s to %s.\n", configuration, path)
		if out.TokenDuration > 0 {
			fmt.Fprintf(console.Stdout(ctx), "Token valid for at least %s.\n", formatDuration(out.TokenDuration))
		}
		style := colors.Ctx(ctx)
		fmt.Fprintf(console.Stdout(ctx), "\nRestart the Buck2 daemon to pick it up:\n  %s\n", style.Highlight.Apply("buck2 killall"))
	}
	return nil
}

func writeCredentialFile(path string, content []byte) error {
	if err := writeAtomicFile(content, path, 0600); err != nil {
		return fnerrors.Newf("failed to write %q: %w", path, err)
	}
	return nil
}
