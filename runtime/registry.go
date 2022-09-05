// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"context"
	"io"
	"net"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/planning"
)

var (
	mapping = map[string]MakeRuntimeFunc{}
)

type MakeRuntimeFunc func(context.Context, planning.Context) (DeferredRuntime, error)

func Register(name string, r MakeRuntimeFunc) {
	mapping[strings.ToLower(name)] = r
}

func HasRuntime(name string) bool {
	_, ok := mapping[strings.ToLower(name)]
	return ok
}

func For(ctx context.Context, env planning.Context) Runtime {
	if obtain, ok := mapping[strings.ToLower(env.Environment().Runtime)]; ok {
		r, err := obtain(ctx, env)
		if err != nil {
			return runtimeFwdErr{err}
		}

		rt, err := r.New(ctx, env)
		if err != nil {
			return runtimeFwdErr{err}
		}

		return rt
	}

	return nil
}

func TargetPlatforms(ctx context.Context, env planning.Context) ([]specs.Platform, error) {
	rt := strings.ToLower(env.Environment().Runtime)
	if obtain, ok := mapping[rt]; ok {
		r, err := obtain(ctx, env)
		if err != nil {
			return nil, err
		}

		if h, ok := r.(HasTargetPlatforms); ok {
			return h.TargetPlatforms(ctx)
		}

		runtime, err := r.New(ctx, env)
		if err != nil {
			return nil, err
		}

		return runtime.TargetPlatforms(ctx)
	}

	return nil, fnerrors.InternalError("%s: no such runtime", rt)
}

func PrepareProvision(ctx context.Context, env planning.Context) (*rtypes.ProvisionProps, error) {
	rt := strings.ToLower(env.Environment().Runtime)
	if obtain, ok := mapping[rt]; ok {
		r, err := obtain(ctx, env)
		if err != nil {
			return nil, err
		}

		if h, ok := r.(HasPrepareProvision); ok {
			return h.PrepareProvision(ctx, env)
		}

		runtime, err := r.New(ctx, env)
		if err != nil {
			return nil, err
		}

		return runtime.PrepareProvision(ctx, env)
	}

	return nil, fnerrors.InternalError("%s: no such runtime", rt)
}

type runtimeFwdErr struct{ err error }

func (r runtimeFwdErr) PrepareProvision(context.Context, planning.Context) (*rtypes.ProvisionProps, error) {
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
func (r runtimeFwdErr) ComputeBaseNaming(context.Context, *schema.Naming) (*schema.ComputedNaming, error) {
	return nil, r.err
}
func (r runtimeFwdErr) FetchLogsTo(context.Context, io.Writer, *ContainerReference, FetchLogsOpts) error {
	return r.err
}
func (r runtimeFwdErr) FetchDiagnostics(context.Context, *ContainerReference) (*Diagnostics, error) {
	return &Diagnostics{}, r.err
}
func (r runtimeFwdErr) FetchEnvironmentDiagnostics(context.Context) (*storage.EnvironmentDiagnostics, error) {
	return nil, r.err
}
func (r runtimeFwdErr) StartTerminal(ctx context.Context, server *schema.Server, io TerminalIO, command string, rest ...string) error {
	return r.err
}
func (r runtimeFwdErr) AttachTerminal(ctx context.Context, _ *ContainerReference, io TerminalIO) error {
	return r.err
}
func (r runtimeFwdErr) ForwardPort(ctx context.Context, server *schema.Server, containerPort int32, localAddrs []string, callback SinglePortForwardedFunc) (io.Closer, error) {
	return nil, r.err
}
func (r runtimeFwdErr) DialServer(ctx context.Context, server *schema.Server, containerPort int32) (net.Conn, error) {
	return nil, r.err
}
func (r runtimeFwdErr) ForwardIngress(ctx context.Context, localAddrs []string, localPort int, f PortForwardedFunc) (io.Closer, error) {
	return nil, r.err
}
func (r runtimeFwdErr) Observe(context.Context, *schema.Server, ObserveOpts, func(ObserveEvent) error) error {
	return r.err
}
func (r runtimeFwdErr) RunOneShot(context.Context, string, ServerRunOpts, io.Writer, bool) error {
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
func (r runtimeFwdErr) ResolveContainers(context.Context, *schema.Server) ([]*ContainerReference, error) {
	return nil, r.err
}
