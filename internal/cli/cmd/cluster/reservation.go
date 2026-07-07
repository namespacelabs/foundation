// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	computev1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/compute/v1beta"
	expcompute "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/experimental/compute"
	"buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/stdlib"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api/private"
	"namespacelabs.dev/foundation/internal/providers/nscloud/endpoint"
	"namespacelabs.dev/foundation/std/tasks"
)

// NewReservationCmd implements `nsc reservation`.
func NewReservationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reservation",
		Short: "Reservation-related activities.",
	}

	cmd.AddCommand(NewReservationCreateCmd("create"))
	cmd.AddCommand(NewReservationWaitCmd())
	cmd.AddCommand(NewReservationListCmd())
	cmd.AddCommand(NewReservationDescribeCmd())
	cmd.AddCommand(NewReservationCancelCmd())

	return cmd
}

const (
	methodReserveInstance     = "namespace.experimental.compute.ReservationService/ReserveInstance"
	methodDescribeReservation = "namespace.experimental.compute.ReservationService/DescribeReservation"
	methodCancelReservation   = "namespace.experimental.compute.ReservationService/CancelReservation"
	methodListReservations    = "namespace.experimental.compute.ReservationService/ListReservations"
)

// callReservation invokes a ReservationService method on the regional public
// endpoint. The experimental ReservationService is served over the Connect/JSON
// transport (the same one used by the private InstanceService), not native gRPC,
// so we serialize protobuf messages with protojson and POST them via fnapi.
func callReservation(ctx context.Context, method string, req, resp proto.Message) error {
	reqBytes, err := protojson.Marshal(req)
	if err != nil {
		return fnerrors.InternalError("failed to marshal request: %w", err)
	}

	call := fnapi.Call[json.RawMessage]{
		IssueBearerToken: fnapi.IssueBearerToken,
		Method:           method,
	}

	return call.Do(ctx, json.RawMessage(reqBytes), endpoint.ResolveRegionalEndpoint, func(r io.Reader) error {
		body, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		return protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(body, resp)
	})
}

// NewReservationListCmd implements `nsc reservation list`.
func NewReservationListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists your reservations.",
		Args:  cobra.NoArgs,
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")
	since := fncobra.Duration(cmd.Flags(), "since", 7*24*time.Hour, "Constrain the list to reservations active within this duration. Set to 0 to list all.")
	maxEntries := cmd.Flags().Int64("max_entries", 100, "Maximum number of reservations to return.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		req := &expcompute.ListReservationsRequest{
			MaxEntries: *maxEntries,
		}

		if *since > 0 {
			req.PeriodStart = timestamppb.New(time.Now().Add(-*since))
		}

		resp := &expcompute.ListReservationsResponse{}
		if err := callReservation(ctx, methodListReservations, req, resp); err != nil {
			return fnerrors.Newf("failed to list reservations: %w", err)
		}

		reservations := resp.GetReservations()
		stdout := console.Stdout(ctx)

		if *output == "json" {
			views := make([]reservationView, 0, len(reservations))
			for _, r := range reservations {
				views = append(views, toReservationView(r))
			}

			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(views); err != nil {
				return fnerrors.InternalError("failed to encode reservations as JSON output: %w", err)
			}

			return nil
		}

		if len(reservations) == 0 {
			fmt.Fprintln(stdout, "No reservations.")
			return nil
		}

		w := tabwriter.NewWriter(stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "RESERVATION ID\tINSTANCE ID\tSTATUS\tRESERVED\tDEADLINE")
		for _, r := range reservations {
			v := toReservationView(r)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", v.ReservationId, orDash(v.InstanceId), v.Status, orDash(v.ReservationTime), orDash(v.ReservationDeadline))
		}

		return w.Flush()
	})

	return cmd
}

// NewReservationDescribeCmd implements `nsc reservation describe <reservation_id>`.
func NewReservationDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe <reservation_id>",
		Short: "Describes a reservation.",
		Args:  cobra.ExactArgs(1),
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		reservationID := args[0]

		resp := &expcompute.DescribeReservationResponse{}
		if err := callReservation(ctx, methodDescribeReservation, &expcompute.DescribeReservationRequest{ReservationId: reservationID}, resp); err != nil {
			return fnerrors.Newf("failed to describe reservation: %w", err)
		}

		v := toReservationView(resp)
		stdout := console.Stdout(ctx)

		if *output == "json" {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(v); err != nil {
				return fnerrors.InternalError("failed to encode reservation as JSON output: %w", err)
			}

			return nil
		}

		w := tabwriter.NewWriter(stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintf(w, "Reservation ID:\t%s\n", v.ReservationId)
		fmt.Fprintf(w, "Status:\t%s\n", v.Status)
		fmt.Fprintf(w, "Instance ID:\t%s\n", orDash(v.InstanceId))
		fmt.Fprintf(w, "Reserved:\t%s\n", orDash(v.ReservationTime))
		fmt.Fprintf(w, "Deadline:\t%s\n", orDash(v.ReservationDeadline))
		fmt.Fprintf(w, "Fulfilled:\t%s\n", orDash(v.FulfillmentTime))

		return w.Flush()
	})

	return cmd
}

// NewReservationCancelCmd implements `nsc reservation cancel <reservation_id>`.
func NewReservationCancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel <reservation_id>",
		Short: "Cancels a pending reservation.",
		Args:  cobra.ExactArgs(1),
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		reservationID := args[0]

		if err := callReservation(ctx, methodCancelReservation, &expcompute.CancelReservationRequest{ReservationId: reservationID}, &expcompute.CancelReservationResponse{}); err != nil {
			return fnerrors.Newf("failed to cancel reservation: %w", err)
		}

		fmt.Fprintf(console.Stdout(ctx), "Canceled reservation %s.\n", reservationID)

		return nil
	})

	return cmd
}

type reservationView struct {
	ReservationId       string `json:"reservation_id"`
	InstanceId          string `json:"instance_id,omitempty"`
	Status              string `json:"status"`
	ReservationTime     string `json:"reservation_time,omitempty"`
	ReservationDeadline string `json:"reservation_deadline,omitempty"`
	FulfillmentTime     string `json:"fulfillment_time,omitempty"`
}

func toReservationView(r *expcompute.DescribeReservationResponse) reservationView {
	v := reservationView{
		ReservationId: r.GetReservationId(),
		InstanceId:    r.GetInstanceId(),
		Status:        strings.ToLower(r.GetStatus().String()),
	}

	if md := r.GetReservationMetadata(); md != nil {
		if t := md.GetReservationTime(); t != nil {
			v.ReservationTime = t.AsTime().Format(time.RFC3339)
		}
		if dl := md.GetReservationDeadline(); dl != nil {
			v.ReservationDeadline = dl.AsTime().Format(time.RFC3339)
		}
	}

	if ft := r.GetFulfillmentTime(); ft != nil {
		v.FulfillmentTime = ft.AsTime().Format(time.RFC3339)
	}

	return v
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// NewReservationWaitCmd implements `nsc reservation wait <reservation_id>`. It
// polls DescribeReservation until the reservation is fulfilled and then outputs
// the instance id.
func NewReservationWaitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wait <reservation_id>",
		Short: "Waits for a reservation to be fulfilled, returning the instance id.",
		Args:  cobra.ExactArgs(1),
	}

	cidfile := cmd.Flags().String("cidfile", "", "If specified, write the instance id to this path.")
	waitTimeout := fncobra.Duration(cmd.Flags(), "wait_timeout", 10*time.Minute, "For how long to wait until the reservation is fulfilled. Set to 0 to wait until the reservation deadline.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		reservationID := args[0]

		if *waitTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, *waitTimeout)
			defer cancel()
		}

		instanceID, err := waitReservation(ctx, reservationID)
		if err != nil {
			return err
		}

		if *cidfile != "" {
			if err := os.WriteFile(*cidfile, []byte(instanceID), 0644); err != nil {
				return fnerrors.Newf("failed to write %q: %w", *cidfile, err)
			}
		}

		fmt.Fprintf(console.Stdout(ctx), "%s\n", instanceID)

		return nil
	})

	return cmd
}

func waitReservation(ctx context.Context, reservationID string) (string, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.reservation-wait").Arg("reservation_id", reservationID).HumanReadable("Waiting for reservation to be fulfilled"), func(ctx context.Context) (string, error) {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			resp := &expcompute.DescribeReservationResponse{}
			if err := callReservation(ctx, methodDescribeReservation, &expcompute.DescribeReservationRequest{ReservationId: reservationID}, resp); err != nil {
				return "", fnerrors.Newf("failed to describe reservation: %w", err)
			}

			if id := resp.GetInstanceId(); id != "" {
				return id, nil
			}

			if md := resp.GetReservationMetadata(); md != nil {
				if dl := md.GetReservationDeadline(); dl != nil && time.Now().After(dl.AsTime()) {
					return "", fnerrors.Newf("reservation %s expired before it was fulfilled", reservationID)
				}
			}

			select {
			case <-ctx.Done():
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					return "", fnerrors.Newf("timed out waiting for reservation %s to be fulfilled", reservationID)
				}
				return "", ctx.Err()
			case <-ticker.C:
			}
		}
	})
}

// waitAndPrintReservation implements `nsc reservation create --wait`. It reuses
// the `nsc reservation wait` code to block until the reservation is fulfilled,
// then waits for the instance to become ready and prints the same output as
// `nsc instance create`.
func waitAndPrintReservation(ctx context.Context, reservationID string, waitTimeout time.Duration, cidfile, output string) error {
	if waitTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, waitTimeout)
		defer cancel()
	}

	instanceID, err := waitReservation(ctx, reservationID)
	if err != nil {
		return err
	}

	if cidfile != "" {
		if err := os.WriteFile(cidfile, []byte(instanceID), 0644); err != nil {
			return fnerrors.Newf("failed to write %q: %w", cidfile, err)
		}
	}

	// The reservation is fulfilled; wait for the instance to become ready and
	// fetch its details, reusing the same wait code as `nsc instance create`.
	// The readiness stream must target the instance's own API endpoint (which
	// `nsc instance create` gets from its create response), so resolve it first.
	info, err := api.GetCluster(ctx, api.Methods, instanceID)
	if err != nil {
		return fnerrors.Newf("failed to fetch instance %s: %w", instanceID, err)
	}

	var apiEndpoint string
	if info.Cluster != nil {
		apiEndpoint = info.Cluster.ApiEndpoint
	}

	clusterWaitFor := waitTimeout
	if clusterWaitFor <= 0 {
		// waitTimeout == 0 means "wait until the reservation deadline"; the
		// reservation is already fulfilled here, so bound the readiness wait.
		clusterWaitFor = 10 * time.Minute
	}

	cluster, err := api.WaitClusterReady(ctx, api.Methods, instanceID, clusterWaitFor, api.WaitClusterOpts{
		WaitKind:    "kubernetes",
		ApiEndpoint: apiEndpoint,
	})
	if err != nil {
		return err
	}

	return printInstanceCreated(ctx, output, reservationID, cluster)
}

// NewReservationCreateCmd implements the reservation creation command. It is
// registered both as `nsc reservation create` and as the `nsc instance reserve`
// alias. It mirrors the flags of `nsc instance create` but calls the public
// ReservationService.ReserveInstance API instead of the private InstanceService.
func NewReservationCreateCmd(use string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: "Reserves a new instance, returning a reservation id.",
		Args:  cobra.NoArgs,
	}

	// creation flags
	machineType := cmd.Flags().String("machine_type", "", "Specify the machine type, in the form [os/arch:]<vcpu>x<memoryGB> (e.g. 4x8 or linux/arm64:2x8). If omitted, the server picks a default shape.")
	duration := fncobra.Duration(cmd.Flags(), "duration", 0, "For how long to run the instance once it is created.")
	bare := cmd.Flags().Bool("bare", false, "If set to true, creates an environment with the minimal set of services (e.g. no Kubernetes).")
	selectors := cmd.Flags().StringSlice("selectors", nil, "Select platform/base image based on specific properties (prop1=value1,prop2=value2).")
	ingress := cmd.Flags().String("ingress", "", "If set, configures the ingress of this instance. Valid options: wildcard.")
	volumes := cmd.Flags().StringSlice("volume", nil, "Attach a volume to the instance, {cache|persistent}:{tag}:{mountpoint}:{size}")
	userSshKey := cmd.Flags().String("ssh_key", "", "Injects the specified ssh public key in the created instance.")
	enable := cmd.Flags().StringSlice("enable", nil, "Enable a feature, e.g. --enable=kubernetes:1.33")
	features := cmd.Flags().StringSlice("features", nil, "A set of features to attach to the instance.")

	// Metadata
	labels := cmd.Flags().StringToString("label", nil, "Key-values to attach to the new instance. Multiple key-value pairs may be specified.")
	purpose := cmd.Flags().String("purpose", "Manually reserved from CLI", "What documented purpose to attach to the created instance.")

	// Output
	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")
	cidfile := cmd.Flags().String("cidfile", "", "When --wait is set, write the instance id to this path.")

	// Reservation-specific
	reservationTimeout := fncobra.Duration(cmd.Flags(), "reservation_timeout", 15*time.Minute, "For how long the server should keep trying to fulfill the reservation before giving up.")
	wait := cmd.Flags().Bool("wait", false, "If set, waits for the reservation to be fulfilled and the instance to become ready, then prints the instance details.")
	waitTimeout := fncobra.Duration(cmd.Flags(), "wait_timeout", 10*time.Minute, "When --wait is set, for how long to wait until the reservation is fulfilled and the instance is ready. Set to 0 to wait until the reservation deadline.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		instanceReq, err := buildReservationInstanceRequest(reservationFlags{
			machineType: *machineType,
			features:    *features,
			bare:        *bare,
			labels:      *labels,
			purpose:     *purpose,
			selectors:   *selectors,
			ingress:     *ingress,
			sshKey:      *userSshKey,
			enable:      *enable,
			volumes:     *volumes,
			duration:    *duration,
		})
		if err != nil {
			return err
		}

		if *reservationTimeout <= 0 {
			return fnerrors.Newf("--reservation_timeout must be greater than zero")
		}

		req := &expcompute.ReserveInstanceRequest{
			CreateInstanceReq:   instanceReq,
			ReservationDeadline: timestamppb.New(time.Now().Add(*reservationTimeout)),
		}

		resp := &expcompute.ReserveInstanceResponse{}
		if err := callReservation(ctx, methodReserveInstance, req, resp); err != nil {
			return fnerrors.Newf("failed to reserve instance: %w", err)
		}

		if *wait {
			// Surface the reservation id before blocking, so it is available even
			// if the wait is interrupted. Skip it for JSON output to keep the
			// final instance JSON parseable.
			if *output != "json" {
				fmt.Fprintf(console.Stdout(ctx), "\n  Created reservation! ID: %s\n", resp.GetReservationId())
			}

			return waitAndPrintReservation(ctx, resp.GetReservationId(), *waitTimeout, *cidfile, *output)
		}

		stdout := console.Stdout(ctx)

		switch *output {
		case "json":
			out := reserveOutput{ReservationId: resp.GetReservationId()}
			if md := resp.GetMetadata(); md != nil {
				if dl := md.GetReservationDeadline(); dl != nil {
					out.ReservationDeadline = dl.AsTime().Format(time.RFC3339)
				}
			}

			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(out); err != nil {
				return fnerrors.InternalError("failed to encode reservation as JSON output: %w", err)
			}

		default:
			if *output != "plain" {
				fmt.Fprintf(console.Warnings(ctx), "defaulting output to plain\n")
			}

			fmt.Fprintf(stdout, "\n  Created reservation! ID: %s\n", resp.GetReservationId())
			fmt.Fprintf(stdout, "\n  To wait for the reservation to get fulfilled and get the instance id, run:\n  nsc reservation wait %s\n\n", resp.GetReservationId())
		}

		return nil
	})

	return cmd
}

type reservationFlags struct {
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

func buildReservationInstanceRequest(flags reservationFlags) (*computev1beta.CreateInstanceRequest, error) {
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

	// Default to single-node Kubernetes on Linux, matching `nsc instance create`.
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

type reserveOutput struct {
	ReservationId       string `json:"reservation_id,omitempty"`
	ReservationDeadline string `json:"reservation_deadline,omitempty"`
}
