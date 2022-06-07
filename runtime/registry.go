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
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
)

var (
	mapping = map[string]MakeRuntimeFunc{}
)

type MakeRuntimeFunc func(context.Context, *schema.Workspace, *schema.DevHost, *schema.Environment) (Runtime, error)

func Register(name string, r MakeRuntimeFunc) {
	mapping[strings.ToLower(name)] = r
}

func HasRuntime(name string) bool {
	_, ok := mapping[strings.ToLower(name)]
	return ok
}

type Selector interface {
	Workspace() *schema.Workspace
	DevHost() *schema.DevHost
	Proto() *schema.Environment
}

func For(ctx context.Context, env Selector) Runtime {
	if obtain, ok := mapping[strings.ToLower(env.Proto().Runtime)]; ok {
		r, err := obtain(ctx, env.Workspace(), env.DevHost(), env.Proto())
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
func (r runtimeFwdErr) FetchLogsTo(context.Context, io.Writer, ContainerReference, FetchLogsOpts) error {
	return r.err
}
func (r runtimeFwdErr) FetchDiagnostics(context.Context, ContainerReference) (Diagnostics, error) {
	return Diagnostics{}, r.err
}
func (r runtimeFwdErr) StartTerminal(ctx context.Context, server *schema.Server, io TerminalIO, command string, rest ...string) error {
	return r.err
}
func (r runtimeFwdErr) AttachTerminal(ctx context.Context, _ ContainerReference, io TerminalIO) error {
	return r.err
}
func (r runtimeFwdErr) ForwardPort(ctx context.Context, server *schema.Server, endpoint *schema.Endpoint, localAddrs []string, callback SinglePortForwardedFunc) (io.Closer, error) {
	return nil, r.err
}
func (r runtimeFwdErr) ForwardIngress(ctx context.Context, localAddrs []string, localPort int, f PortForwardedFunc) (io.Closer, error) {
	return nil, r.err
}
func (r runtimeFwdErr) Observe(context.Context, *schema.Server, ObserveOpts, func(ObserveEvent) error) error {
	return r.err
}
func (r runtimeFwdErr) RunOneShot(context.Context, schema.PackageName, ServerRunOpts, io.Writer) error {
	return r.err
}
func (r runtimeFwdErr) RunAttached(context.Context, string, ServerRunOpts, TerminalIO) error {
	return r.err
}
func (r runtimeFwdErr) DeleteRecursively(context.Context, bool) (bool, error) {
	return false, r.err
}
func (r runtimeFwdErr) DeleteAllRecursively(context.Context, bool, io.Writer) (bool, error) {
	return false, r.err
}
func (r runtimeFwdErr) TargetPlatforms(context.Context) ([]specs.Platform, error) {
	return nil, r.err
}
func (r runtimeFwdErr) ResolveContainers(context.Context, *schema.Server) ([]ContainerReference, error) {
	return nil, r.err
}
