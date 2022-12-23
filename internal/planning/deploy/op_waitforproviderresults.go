// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/framework/resources/provider"
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

var (
	resultHeader  = []byte("namespace.provision.result:")
	messageHeader = []byte("namespace.provision.message:")
)

func register_OpWaitForProviderResults() {
	execution.RegisterFuncs(execution.Funcs[*internalres.OpWaitForProviderResults]{
		EmitStart: func(ctx context.Context, inv *schema.SerializedInvocation, wait *internalres.OpWaitForProviderResults, ch chan *orchpb.Event) {
			var label string
			if wait.GetResourceClass().GetDescription() != "" {
				label = fmt.Sprintf("%s (%s)", wait.GetResourceClass().GetDescription(), wait.ResourceInstanceId)
			}

			ch <- &orchpb.Event{
				ResourceId:    wait.ResourceInstanceId,
				Category:      "Resources deployed",
				Ready:         orchpb.Event_NOT_READY,
				Stage:         orchpb.Event_WAITING,
				ResourceLabel: label,
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
						Stage:      orchpb.Event_WAITING,
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
				if err := cluster.Cluster().FetchLogsTo(ctx, main.Reference, runtime.FetchLogsOpts{}, runtime.WriteToWriter(&out)); err != nil {
					return nil, fnerrors.InternalError("failed to retrieve output of provider invocation: %w", err)
				}

				if main.TerminationError != nil {
					fmt.Fprintf(console.Errors(ctx), "%s provision failure:\n%s\n", wait.ResourceInstanceId, out.Bytes())

					return nil, fnerrors.ExternalError("provider failed: %w\n\n    >> See the logs above for the provider error. <<\n", main.TerminationError)
				}

				_, msgdesc, err := protos.LoadMessageByName(wait.InstanceTypeSource, wait.ResourceClass.InstanceType.ProtoType)
				if err != nil {
					return nil, err
				}

				tasks.Attachments(ctx).Attach(tasks.Output("invocation-output.log", "text/plain"), out.Bytes())

				// The protocol is that a provision tool must emit a line `namespace.provision.message: json` (or message.provision.result previously).
				lines := bytes.Split(out.Bytes(), []byte("\n"))

				var resultMessage proto.Message

				setMessage := func(serialized []byte) error {
					if resultMessage != nil {
						return fnerrors.ExternalError("invocation produced multiple results")
					}

					parsedMessage := dynamicpb.NewMessage(msgdesc).Interface()
					if err := protojson.Unmarshal(serialized, parsedMessage); err != nil {
						return fnerrors.ExternalError("failed to unmarshal provider result: %w", err)
					}

					resultMessage = parsedMessage
					return nil
				}

				for _, line := range lines {
					switch {
					case bytes.HasPrefix(line, resultHeader):
						original := bytes.TrimPrefix(line, resultHeader)
						if err := setMessage(original); err != nil {
							return nil, err
						}

					case bytes.HasPrefix(line, messageHeader):
						var msg provider.Message
						original := bytes.TrimPrefix(line, messageHeader)
						if err := json.Unmarshal(original, &msg); err != nil {
							return nil, fnerrors.ExternalError("failed to unmarshal provider message: %w", err)
						}

						if msg.SerializedInstanceJSON != nil {
							if err := setMessage([]byte(*msg.SerializedInstanceJSON)); err != nil {
								return nil, err
							}
						}
					}
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
						Stage:      orchpb.Event_DONE,
						Timestamp:  timestamppb.Now(),
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
