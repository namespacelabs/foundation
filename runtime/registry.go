// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"context"
	"io"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
)

var (
	mapping = map[string]MakeRuntimeFunc{}
)

type MakeRuntimeFunc func(*schema.Workspace, *schema.DevHost, *schema.Environment) (Runtime, error)

func Register(name string, r MakeRuntimeFunc) {
	mapping[strings.ToLower(name)] = r
}

func For(env ops.Environment) Runtime {
	return ForProto(env.Workspace(), env.Proto(), env.DevHost())
}

func ForProto(ws *schema.Workspace, env *schema.Environment, devHost *schema.DevHost) Runtime {
	if make, ok := mapping[strings.ToLower(env.Runtime)]; ok {
		r, err := make(ws, devHost, env)
		if err != nil {
			return runtimeFwdErr{err}
		}

		return r
	}

	return nil
}

type runtimeFwdErr struct{ err error }

func (r runtimeFwdErr) PrepareProvision(context.Context) (*rtypes.ProvisionProps, error) {
	return nil, r.err
}
func (r runtimeFwdErr) DeployedConfigImageID(context.Context, *schema.Server) (oci.ImageID, error) {
	return oci.ImageID{}, r.err
}
func (r runtimeFwdErr) PlanDeployment(context.Context, Deployment) (DeploymentState, error) {
	return nil, r.err
}
func (r runtimeFwdErr) PlanIngress(context.Context, *schema.Stack, []*schema.IngressFragment) (DeploymentState, error) {
	return nil, r.err
}
func (r runtimeFwdErr) PlanShutdown(context.Context, []provision.Server, []provision.Server) ([]*schema.Definition, error) {
	return nil, r.err
}
func (r runtimeFwdErr) StreamLogsTo(context.Context, io.Writer, *schema.Server, StreamLogsOpts) error {
	return r.err
}
func (r runtimeFwdErr) StartTerminal(ctx context.Context, server *schema.Server, io TerminalIO, command string, rest ...string) error {
	return r.err
}
func (r runtimeFwdErr) ForwardPort(ctx context.Context, server *schema.Server, endpoint *schema.Endpoint, localAddrs []string, callback SinglePortForwardedFunc) (io.Closer, error) {
	return nil, r.err
}
func (r runtimeFwdErr) ForwardIngress(ctx context.Context, localAddrs []string, localPort int, f PortForwardedFunc) (io.Closer, error) {
	return nil, r.err
}
func (r runtimeFwdErr) Observe(context.Context, provision.Server, ObserveOpts, func(ObserveEvent) error) error {
	return r.err
}

func (r runtimeFwdErr) RunOneShot(context.Context, schema.PackageName, ServerRunOpts, io.Writer) error {
	return r.err
}

func (r runtimeFwdErr) DeleteRecursively(context.Context) error {
	return r.err
}

func (r runtimeFwdErr) DebugShell(ctx context.Context, img oci.ImageID, io rtypes.IO) error {
	return r.err
}

func (r runtimeFwdErr) HostPlatforms() []specs.Platform {
	return nil
}