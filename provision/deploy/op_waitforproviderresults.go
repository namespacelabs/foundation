// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"bytes"
	"context"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
	"namespacelabs.dev/foundation/engine/ops"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	internalres "namespacelabs.dev/foundation/internal/resources"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/source/protos"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var GarbageCollectProviders = true

var resultHeader = []byte("namespace.provision.result:")

func register_OpWaitForProviderResults() {
	ops.RegisterHandlerFunc(func(ctx context.Context, inv *schema.SerializedInvocation, wait *internalres.OpWaitForProviderResults) (*ops.HandleResult, error) {
		action := tasks.Action("resource.complete-invocation").
			Scope(schema.PackageName(wait.Deployable.PackageName)).
			Arg("resource_instance_id", wait.ResourceInstanceId).
			HumanReadablef(inv.Description)

		return tasks.Return(ctx, action, func(ctx context.Context) (*ops.HandleResult, error) {
			cluster, err := ops.Get(ctx, runtime.ClusterNamespaceInjection)
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

				return nil, fnerrors.InvocationError("provider failed: %w", main.TerminationError)
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
					return nil, fnerrors.InternalError("invocation produced multiple results")
				}

				parsedMessage := dynamicpb.NewMessage(msgdesc).Interface()
				original := bytes.TrimPrefix(line, resultHeader)
				if err := protojson.Unmarshal(original, parsedMessage); err != nil {
					return nil, fnerrors.InvocationError("failed to unmarshal provision result: %w", err)
				}

				resultMessage = parsedMessage
			}

			if resultMessage == nil {
				return nil, fnerrors.InvocationError("provision did not produce a result")
			}

			_ = tasks.Attachments(ctx).AttachSerializable("instance.json", "", resultMessage)

			return &ops.HandleResult{
				Outputs: []ops.Output{
					{InstanceID: wait.ResourceInstanceId, Message: resultMessage},
				},
			}, nil
		})
	})
}
