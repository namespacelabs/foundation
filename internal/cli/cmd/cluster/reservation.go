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
	"strings"
	"text/tabwriter"
	"time"

	expcompute "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/experimental/compute"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/endpoint"
	"namespacelabs.dev/foundation/std/tasks"
)

// NewReservationCmd implements `nsc reservation`.
func NewReservationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reservation",
		Short: "Manage instance reservations.",
	}

	cmd.AddCommand(NewReservationWaitCmd())
	cmd.AddCommand(NewReservationListCmd())
	cmd.AddCommand(NewReservationDescribeCmd())
	cmd.AddCommand(NewReservationCancelCmd())

	return cmd
}

const (
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
		Short: "List your reservations.",
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
		Short: "Describe a reservation.",
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
		Short: "Cancel a pending reservation.",
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
		Short: "Wait for a reservation and print its instance ID.",
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
