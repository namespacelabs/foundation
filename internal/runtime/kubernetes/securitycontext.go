// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"io/fs"
	"strconv"

	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"sigs.k8s.io/yaml"
)

func makeSecurityContext(opts runtime.ContainerRunOpts) (*applycorev1.SecurityContextApplyConfiguration, error) {
	secCtx := applycorev1.SecurityContext()

	const path = "defaults/container.securitycontext.yaml"
	contents, err := fs.ReadFile(defaults, path)
	if err != nil {
		return nil, fnerrors.InternalError("internal kubernetes data failed to read: %w", err)
	}

	if err := yaml.Unmarshal(contents, secCtx); err != nil {
		return nil, fnerrors.InternalError("%s: failed to parse defaults: %w", path, err)
	}

	if opts.Privileged {
		secCtx = secCtx.WithPrivileged(true).WithAllowPrivilegeEscalation(true)
	}

	if opts.ReadOnlyFilesystem {
		secCtx = secCtx.WithReadOnlyRootFilesystem(true)
	}

	return secCtx, nil
}

func runAsToPodSecCtx(podSecCtx *applycorev1.PodSecurityContextApplyConfiguration, runAs *runtime.RunAs) (*applycorev1.PodSecurityContextApplyConfiguration, error) {
	if runAs != nil {
		if runAs.UserID != "" {
			userId, err := strconv.ParseInt(runAs.UserID, 10, 64)
			if err != nil {
				return nil, fnerrors.InternalError("expected server.RunAs.UserID to be an int64: %w", err)
			}

			if podSecCtx.RunAsUser != nil && *podSecCtx.RunAsUser != userId {
				return nil, fnerrors.BadInputError("incompatible userid %d vs %d (in RunAs)", *podSecCtx.RunAsUser, userId)
			}

			podSecCtx = podSecCtx.WithRunAsUser(userId).WithRunAsNonRoot(true)
		}

		if runAs.FSGroup != nil {
			fsGroup, err := strconv.ParseInt(*runAs.FSGroup, 10, 64)
			if err != nil {
				return nil, fnerrors.InternalError("expected server.RunAs.FSGroup to be an int64: %w", err)
			}

			if podSecCtx.FSGroup != nil && *podSecCtx.FSGroup != fsGroup {
				return nil, fnerrors.BadInputError("incompatible fsgroup %d vs %d (in RunAs)", *podSecCtx.FSGroup, fsGroup)
			}

			podSecCtx.WithFSGroup(fsGroup)
		}

		return podSecCtx, nil
	}

	return nil, nil
}
