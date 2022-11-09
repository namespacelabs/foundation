// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"bytes"
	"context"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
	"namespacelabs.dev/foundation/internal/codegen/protos"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	internalres "namespacelabs.dev/foundation/internal/resources"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	orchpb "namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/tasks"
)

var GarbageCollectProviders = true

var resultHeader = []byte("namespace.provision.result:")

func register_OpWaitForProviderResults() {
	execution.RegisterFuncs(execution.Funcs[*internalres.OpWaitForProviderResults]{
		EmitStart: func(ctx context.Context, inv *schema.SerializedInvocation, wait *internalres.OpWaitForProviderResults, ch chan *orchpb.Event) {
			ch <- &orchpb.Event{
				ResourceId: wait.ResourceInstanceId,
				Category:   "Resources deployed",
				Ready:      orchpb.Event_NOT_READY,
			}
		},

		HandleWithEvents: func(ctx context.Context, inv *schema.SerializedInvocation, wait *internalres.OpWaitForProviderResults, ch chan *orchpb.Event) (*execution.HandleResult, error) {
			action := tasks.Action("resource.complete-invocation").
				Scope(wait.Deployable.GetPackageRef().AsPackageName()).
				Arg("resource_instance_id", wait.ResourceInstanceId).
				HumanReadablef(inv.Description)

			return tasks.Return(ctx, action, func(ctx context.Context) (*execution.HandleResult, error) {
				cluster, err := execution.Get(ctx, runtime.ClusterNamespaceInjection)
				if err != nil {
					return nil, err
				}

				if GarbageCollectProviders {
					defer func() {
						if err := cluster.DeleteDeployable(ctx, wait.Deployable); err != nil {
							fmt.Fprintf(console.Errors(ctx), "Deleting %s failed: %v\n", wait.Deployable.Name, err)
						}
					}()
				}

				if ch != nil {
					ch <- &orchpb.Event{
						ResourceId: wait.ResourceInstanceId,
						Category:   "Resources deployed",
						Ready:      orchpb.Event_NOT_READY,
						WaitStatus: []*orchpb.Event_WaitStatus{{Description: "Waiting for provider..."}},
					}
				}

				// XXX add a maximum time we're willing to wait.
				containers, err := cluster.WaitForTermination(ctx, wait.Deployable)
				if err != nil {
					return nil, err
				}

				if len(containers) != 1 {
					return nil, fnerrors.InternalError("expected exactly one container, got %d", len(containers))
				}

				main := containers[0]

				var out bytes.Buffer
				if err := cluster.Cluster().FetchLogsTo(ctx, &out, main.Reference, runtime.FetchLogsOpts{}); err != nil {
					return nil, fnerrors.InternalError("failed to retrieve output of provider invocation: %w", err)
				}

				if main.TerminationError != nil {
					fmt.Fprintf(console.Errors(ctx), "%s provision failure:\n%s\n", wait.ResourceInstanceId, out.Bytes())

					return nil, fnerrors.ExternalError("provider failed: %w", main.TerminationError)
				}

				_, msgdesc, err := protos.LoadMessageByName(wait.InstanceTypeSource, wait.ResourceClass.InstanceType.ProtoType)
				if err != nil {
					return nil, err
				}

				tasks.Attachments(ctx).Attach(tasks.Output("invocation-output.log", "text/plain"), out.Bytes())

				// The protocol is that a provision tool must emit a line `namespace.provision.result: json`
				lines := bytes.Split(out.Bytes(), []byte("\n"))

				var resultMessage proto.Message
				for _, line := range lines {
					if !bytes.HasPrefix(line, resultHeader) {
						continue
					}

					if resultMessage != nil {
						return nil, fnerrors.ExternalError("invocation produced multiple results")
					}

					parsedMessage := dynamicpb.NewMessage(msgdesc).Interface()
					original := bytes.TrimPrefix(line, resultHeader)
					if err := protojson.Unmarshal(original, parsedMessage); err != nil {
						return nil, fnerrors.ExternalError("failed to unmarshal provision result: %w", err)
					}

					resultMessage = parsedMessage
				}

				if resultMessage == nil {
					return nil, fnerrors.ExternalError("provision did not produce a result")
				}

				_ = tasks.Attachments(ctx).AttachSerializable("instance.json", "", resultMessage)

				if ch != nil {
					ch <- &orchpb.Event{
						ResourceId: wait.ResourceInstanceId,
						Category:   "Resources deployed",
						Ready:      orchpb.Event_READY,
					}
				}

				return &execution.HandleResult{
					Outputs: []execution.Output{
						{InstanceID: wait.ResourceInstanceId, Message: resultMessage},
					},
				}, nil
			})
		},
	})
}
