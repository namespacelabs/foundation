// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubectl

import (
	"context"
	"os"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

type Kubeconfig struct {
	Kubeconfig string
	Context    string
	Namespace  string
	keepConfig bool
}

func WriteRawKubeconfig(ctx context.Context, rawConfig clientcmdapi.Config, keepConfig bool) (*Kubeconfig, error) {
	configBytes, err := clientcmd.Write(rawConfig)
	if err != nil {
		return nil, fnerrors.Newf("failed to serialize kubeconfig: %w", err)
	}

	return WriteKubeconfig(ctx, configBytes, keepConfig)
}

func WriteKubeconfig(ctx context.Context, configBytes []byte, keepConfig bool) (*Kubeconfig, error) {
	tmpFile, err := dirs.CreateUserTemp("kubeconfig", "*.yaml")
	if err != nil {
		return nil, fnerrors.Newf("failed to create temp file: %w", err)
	}

	if _, err := tmpFile.Write(configBytes); err != nil {
		return nil, fnerrors.Newf("failed to write kubeconfig: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return nil, fnerrors.Newf("failed to close kubeconfig: %w", err)
	}

	return &Kubeconfig{
		Kubeconfig: tmpFile.Name(),
		keepConfig: keepConfig,
	}, nil
}

func (kc *Kubeconfig) BaseArgs() []string {
	baseArgs := []string{
		"--kubeconfig=" + kc.Kubeconfig,
	}

	if kc.Namespace != "" {
		baseArgs = append(baseArgs, "-n", kc.Namespace)
	}

	if kc.Context != "" {
		baseArgs = append(baseArgs, "--context", kc.Context)
	}

	return baseArgs
}

func (kc *Kubeconfig) Cleanup() error {
	if kc.keepConfig {
		return nil
	}

	return os.Remove(kc.Kubeconfig)
}
