// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"bytes"
	"context"
	"encoding/json"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
	"namespacelabs.dev/foundation/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	internalres "namespacelabs.dev/foundation/internal/resources"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/source/protos"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func Register_OpWaitForProviderResults() {
	ops.RegisterHandlerFunc(func(ctx context.Context, inv *schema.SerializedInvocation, wait *internalres.OpWaitForProviderResults) (*ops.HandleResult, error) {
		cluster, err := ops.Get(ctx, runtime.ClusterNamespaceInjection)
		if err != nil {
			return nil, err
		}

		// XXX add a maximum time we're willing to wait.
		containers, err := cluster.WaitForTermination(ctx, wait.Deployable)
		if err != nil {
			return nil, err
		}

		if len(containers) != 1 {
			return nil, fnerrors.InternalError("expected exactly one container, got %d", len(containers))
		}

		var out bytes.Buffer

		if err := cluster.Cluster().FetchLogsTo(ctx, &out, containers[0].Reference, runtime.FetchLogsOpts{}); err != nil {
			return nil, fnerrors.InternalError("failed to retrieve output of provider invocation: %w", err)
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
			if !bytes.HasPrefix(line, []byte("namespace.provision.result:")) {
				continue
			}

			if resultMessage != nil {
				return nil, fnerrors.InternalError("invocation produced multiple results")
			}

			parsedMessage := dynamicpb.NewMessage(msgdesc).Interface()
			result := bytes.TrimPrefix(line, []byte("namespace.provision.result:"))
			if err := json.Unmarshal(result, parsedMessage); err != nil {
				return nil, fnerrors.InvocationError("failed to unmarshal provision result: %w", err)
			}

			resultMessage = parsedMessage
		}

		if resultMessage == nil {
			return nil, fnerrors.InvocationError("provision did not produce a result")
		}

		_ = tasks.Attachments(ctx).AttachSerializable("invocation-output.json", "", resultMessage)

		return &ops.HandleResult{
			Outputs: []ops.Output{
				{Key: wait.ResultId, Message: resultMessage},
			},
		}, nil
	})
}
