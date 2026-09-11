// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"strconv"
	"strings"
	"time"

	computev1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/compute/v1beta"
	expcompute "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/experimental/compute"
	"buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/stdlib"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api/private"
)

// This file implements the public compute API path for `nsc create`, gated
// behind --use_public_api. It is intentionally self-contained: instead of the
// private InstanceService (api.CreateAndWaitCluster), it reserves an instance
// through the public ReservationService.ReserveInstance API and waits for it to
// become ready. Once the legacy private path is removed, deleting the else
// branch in NewCreateCmd and promoting this path is all that is required.

const methodReserveInstance = "namespace.experimental.compute.ReservationService/ReserveInstance"

// reservationFulfillmentWindow caps how long the server is asked to keep trying
// to fulfill a reservation. The effective deadline is never later than the
// client-side wait (waitTimeout), so the server never keeps working after we
// have given up.
const reservationFulfillmentWindow = 15 * time.Minute

// publicAPIUnsupportedFlags lists `nsc create` flags the public compute API path
// does not map yet. Setting any of them together with --use_public_api is an
// error rather than a silent no-op. Remove entries as mappings are added.
var publicAPIUnsupportedFlags = []string{
	"unique_tag",
	"available_secrets",
	"internal_extra",
	"experimental",
	"experimental_from",
	"experimental_instance_features",
	"experimental_additional_workload_permissions",
}

// ensurePublicAPISupportsFlags fails if the caller explicitly set a flag the
// public compute API path cannot honor, so its meaning is never silently
// dropped.
func ensurePublicAPISupportsFlags(cmd *cobra.Command) error {
	var set []string
	for _, name := range publicAPIUnsupportedFlags {
		if cmd.Flags().Changed(name) {
			set = append(set, "--"+name)
		}
	}

	if len(set) > 0 {
		return fnerrors.Newf("--use_public_api does not support %s yet", strings.Join(set, ", "))
	}

	return nil
}

// createInstanceFlags is the subset of `nsc create` flags that the public
// compute API path understands. It mirrors the creation-related flags of
// `nsc create`.
type createInstanceFlags struct {
	machineType string
	features    []string
	bare        bool
	labels      map[string]string
	purpose     string
	selectors   []string
	ingress     string
	sshKey      string
	enable      []string
	volumes     []string
	duration    time.Duration
}

// createInstanceViaPublicAPI reserves an instance through the public compute
// ReservationService.ReserveInstance API, waits for the reservation to be
// fulfilled and the instance to become ready, and returns the cluster result.
// It is the public-API counterpart to the legacy api.CreateAndWaitCluster path.
func createInstanceViaPublicAPI(ctx context.Context, flags createInstanceFlags, waitTimeout time.Duration) (*api.CreateClusterResult, error) {
	// Reservations are asynchronous, so unlike the legacy path this one cannot
	// return an instance without waiting for fulfillment. Require a bounded wait
	// so we can always cancel a reservation we stop waiting for.
	if waitTimeout <= 0 {
		return nil, fnerrors.Newf("--use_public_api requires a positive --wait_timeout, as it must wait for the instance to be created")
	}

	instanceReq, err := buildCreateInstanceRequest(flags)
	if err != nil {
		return nil, err
	}

	// Bound the whole reserve+wait sequence by waitTimeout, matching the legacy
	// path where waitTimeout bounds instance readiness.
	ctx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()

	// The reservation deadline must not outlive our own wait: otherwise the
	// server could still fulfill it (creating an instance) after we have given
	// up, leaving an instance the caller cannot track. Cap it at the fulfillment
	// window, but never beyond our wait budget.
	reservationDeadline := time.Now().Add(reservationFulfillmentWindow)
	if d, ok := ctx.Deadline(); ok && d.Before(reservationDeadline) {
		reservationDeadline = d
	}

	req := &expcompute.ReserveInstanceRequest{
		CreateInstanceReq:   instanceReq,
		ReservationDeadline: timestamppb.New(reservationDeadline),
	}

	resp := &expcompute.ReserveInstanceResponse{}
	if err := callReservation(ctx, methodReserveInstance, req, resp); err != nil {
		return nil, fnerrors.Newf("failed to reserve instance: %w", err)
	}

	reservationID := resp.GetReservationId()

	instanceID, err := waitReservation(ctx, reservationID)
	if err != nil {
		// We gave up before the reservation was fulfilled; cancel it best-effort
		// so the server does not create an instance we can no longer track.
		cancelReservationBestEffort(ctx, reservationID)
		return nil, err
	}

	// The readiness stream must target the instance's own API endpoint, so
	// resolve it before waiting for readiness.
	info, err := api.GetCluster(ctx, api.Methods, instanceID)
	if err != nil {
		return nil, fnerrors.Newf("failed to fetch instance %s: %w", instanceID, err)
	}

	var apiEndpoint string
	if info.Cluster != nil {
		apiEndpoint = info.Cluster.ApiEndpoint
	}

	// ctx already bounds this by the remaining wait budget; waitTimeout is the
	// upper bound, whichever is shorter wins.
	return api.WaitClusterReady(ctx, api.Methods, instanceID, waitTimeout, api.WaitClusterOpts{
		WaitKind:    "kubernetes",
		ApiEndpoint: apiEndpoint,
	})
}

// cancelReservationBestEffort cancels a not-yet-fulfilled reservation, ignoring
// errors. It uses a detached context so cancellation still runs after the
// creation context has expired.
func cancelReservationBestEffort(ctx context.Context, reservationID string) {
	if reservationID == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()

	_ = callReservation(ctx, methodCancelReservation, &expcompute.CancelReservationRequest{ReservationId: reservationID}, &expcompute.CancelReservationResponse{})
}

// buildCreateInstanceRequest translates the CLI flags into a public compute
// CreateInstanceRequest, mirroring how the legacy path builds
// api.CreateInstanceOpts.
func buildCreateInstanceRequest(flags createInstanceFlags) (*computev1beta.CreateInstanceRequest, error) {
	req := &computev1beta.CreateInstanceRequest{
		DocumentedPurpose: flags.purpose,
	}

	exp := &computev1beta.CreateInstanceRequest_ExperimentalFeatures{
		PrivateFeature: flags.features,
	}

	// Shape.
	var shape *computev1beta.InstanceShape
	if flags.machineType != "" {
		os, arch, vcpu, memoryMB, err := ParseMachineTypeShape(flags.machineType)
		if err != nil {
			return nil, err
		}

		shape = &computev1beta.InstanceShape{
			VirtualCpu:      vcpu,
			MemoryMegabytes: memoryMB,
			MachineArch:     arch,
			Os:              os,
		}
	}

	for _, s := range flags.selectors {
		k, v, ok := strings.Cut(s, "=")
		if !ok {
			return nil, fnerrors.Newf("invalid selector %q: expected key=value", s)
		}

		if shape == nil {
			shape = &computev1beta.InstanceShape{}
		}

		shape.Selectors = append(shape.Selectors, &stdlib.Label{Name: k, Value: v})
	}

	req.Shape = shape

	// Labels.
	labels := flags.labels
	if len(labels) == 0 {
		labels = map[string]string{"nsc.source": "nsc"}
	}

	labelKeys := maps.Keys(labels)
	slices.Sort(labelKeys)
	for _, key := range labelKeys {
		req.Labels = append(req.Labels, &stdlib.Label{Name: key, Value: labels[key]})
	}

	// SSH keys.
	keys, err := parseAuthorizedKeys(flags.sshKey)
	if err != nil {
		return nil, err
	}
	exp.AuthorizedSshKeys = keys

	// Volumes.
	for _, def := range flags.volumes {
		spec, err := ParseVolumeFlag(def)
		if err != nil {
			return nil, err
		}

		req.Volumes = append(req.Volumes, &computev1beta.VolumeRequest{
			MountPoint:      spec.MountPoint,
			Tag:             spec.Tag,
			SizeMb:          spec.SizeMb,
			PersistencyKind: volumePersistencyKind(spec.PersistencyKind),
		})
	}

	if flags.bare {
		exp.PrivateFeature = append(exp.PrivateFeature, "EXP_DISABLE_KUBERNETES")
	}

	// Default to single-node Kubernetes on Linux, matching `nsc create`.
	if !flags.bare && !strings.HasPrefix(flags.machineType, "mac") && !strings.HasPrefix(flags.machineType, "windows") {
		req.FeatureConfiguration = &computev1beta.CreateInstanceRequest_FeatureConfiguration{
			EnableKubernetesVersion: private.K3sVersion,
		}
	}

	for _, feat := range flags.enable {
		parts := strings.SplitN(feat, ":", 2)
		switch parts[0] {
		case "kubernetes":
			if len(parts) != 2 {
				return nil, fnerrors.Newf("expected Kubernetes version spec %q", feat)
			}

			if req.FeatureConfiguration == nil {
				req.FeatureConfiguration = &computev1beta.CreateInstanceRequest_FeatureConfiguration{}
			}
			req.FeatureConfiguration.EnableKubernetesVersion = parts[1]

		case "kubernetes-max-pods":
			if len(parts) != 2 {
				return nil, fnerrors.Newf("expected kubernetes-max-pods value %q", feat)
			}

			n, err := strconv.ParseInt(parts[1], 10, 32)
			if err != nil {
				return nil, fnerrors.Newf("invalid kubernetes-max-pods value %q: %w", parts[1], err)
			}

			if req.FeatureConfiguration == nil {
				return nil, fnerrors.Newf("kubernetes-max-pods requires Kubernetes to be enabled")
			}
			req.FeatureConfiguration.KubernetesMaxPods = int32(n)

		default:
			return nil, fnerrors.Newf("unknown feature option %q", parts[0])
		}
	}

	switch flags.ingress {
	case "":
		// nothing to do

	case "wildcard":
		exp.EnableWildcardDomain = true

	default:
		return nil, fnerrors.Newf("unknown ingress option %q", flags.ingress)
	}

	if flags.duration > 0 {
		req.Deadline = timestamppb.New(time.Now().Add(flags.duration))
	}

	req.Experimental = exp

	return req, nil
}

func volumePersistencyKind(kind api.VolumeSpec_PersistencyKind) computev1beta.VolumeRequest_PersistencyKind {
	switch kind {
	case api.VolumeSpec_PERSISTENT:
		return computev1beta.VolumeRequest_PERSISTENT
	case api.VolumeSpec_CACHE:
		return computev1beta.VolumeRequest_CACHE
	default:
		return computev1beta.VolumeRequest_PERSISTENCY_UNKNOWN
	}
}
