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
	registrations = map[string]MakeRuntimeFunc{}
)

type MakeRuntimeFunc func(context.Context, planning.Context) (DeferredRuntime, error)

func Register(name string, r MakeRuntimeFunc) {
	registrations[strings.ToLower(name)] = r
}

func HasRuntime(name string) bool {
	_, ok := registrations[strings.ToLower(name)]
	return ok
}

// Never returns nil. If the specified runtime kind doesn't exist, then a runtime instance that always fails is returned.
func For(ctx context.Context, env planning.Context) Runtime {
	runtime, err := obtainSpecialized[Runtime](ctx, env)
	if err != nil {
		return runtimeFwdErr{err}
	}
	return runtime
}

func TargetPlatforms(ctx context.Context, env planning.Context) ([]specs.Platform, error) {
	runtime, err := obtainSpecialized[HasTargetPlatforms](ctx, env)
	if err != nil {
		return nil, err
	}
	return runtime.TargetPlatforms(ctx)
}

func PrepareProvision(ctx context.Context, env planning.Context) (*rtypes.ProvisionProps, error) {
	runtime, err := obtainSpecialized[HasPrepareProvision](ctx, env)
	if err != nil {
		return nil, err
	}
	return runtime.PrepareProvision(ctx, env)
}

func obtainSpecialized[V any](ctx context.Context, env planning.Context) (V, error) {
	var empty V
	rt := strings.ToLower(env.Environment().Runtime)
	if obtain, ok := registrations[rt]; ok {
		r, err := obtain(ctx, env)
		if err != nil {
			return empty, err
		}

		if h, ok := r.(V); ok {
			return h, nil
		}

		runtime, err := r.New(ctx, env)
		if err != nil {
			return empty, err
		}

		return runtime.(V), nil
	}

	return empty, fnerrors.InternalError("%s: no such runtime", rt)
}

type runtimeFwdErr struct {
	permanentErr error
}

func (r runtimeFwdErr) PrepareProvision(context.Context, planning.Context) (*rtypes.ProvisionProps, error) {
	return nil, r.permanentErr
}
func (r runtimeFwdErr) DeployedConfigImageID(context.Context, *schema.Server) (oci.ImageID, error) {
	return oci.ImageID{}, r.permanentErr
}
func (r runtimeFwdErr) PlanDeployment(context.Context, Deployment) (DeploymentState, error) {
	return nil, r.permanentErr
}
func (r runtimeFwdErr) PlanIngress(context.Context, *schema.Stack, []*schema.IngressFragment) (DeploymentState, error) {
	return nil, r.permanentErr
}
func (r runtimeFwdErr) ComputeBaseNaming(context.Context, *schema.Naming) (*schema.ComputedNaming, error) {
	return nil, r.permanentErr
}
func (r runtimeFwdErr) FetchLogsTo(context.Context, io.Writer, *ContainerReference, FetchLogsOpts) error {
	return r.permanentErr
}
func (r runtimeFwdErr) FetchDiagnostics(context.Context, *ContainerReference) (*Diagnostics, error) {
	return &Diagnostics{}, r.permanentErr
}
func (r runtimeFwdErr) FetchEnvironmentDiagnostics(context.Context) (*storage.EnvironmentDiagnostics, error) {
	return nil, r.permanentErr
}
func (r runtimeFwdErr) StartTerminal(ctx context.Context, server *schema.Server, io TerminalIO, command string, rest ...string) error {
	return r.permanentErr
}
func (r runtimeFwdErr) AttachTerminal(ctx context.Context, _ *ContainerReference, io TerminalIO) error {
	return r.permanentErr
}
func (r runtimeFwdErr) ForwardPort(ctx context.Context, server *schema.Server, containerPort int32, localAddrs []string, callback SinglePortForwardedFunc) (io.Closer, error) {
	return nil, r.permanentErr
}
func (r runtimeFwdErr) DialServer(ctx context.Context, server *schema.Server, containerPort int32) (net.Conn, error) {
	return nil, r.permanentErr
}
func (r runtimeFwdErr) ForwardIngress(ctx context.Context, localAddrs []string, localPort int, f PortForwardedFunc) (io.Closer, error) {
	return nil, r.permanentErr
}
func (r runtimeFwdErr) Observe(context.Context, *schema.Server, ObserveOpts, func(ObserveEvent) error) error {
	return r.permanentErr
}
func (r runtimeFwdErr) RunOneShot(context.Context, string, ServerRunOpts, io.Writer, bool) error {
	return r.permanentErr
}
func (r runtimeFwdErr) RunAttached(context.Context, string, ServerRunOpts, TerminalIO) error {
	return r.permanentErr
}
func (r runtimeFwdErr) DeleteRecursively(context.Context, bool) (bool, error) {
	return false, r.permanentErr
}
func (r runtimeFwdErr) DeleteAllRecursively(context.Context, bool, io.Writer) (bool, error) {
	return false, r.permanentErr
}
func (r runtimeFwdErr) TargetPlatforms(context.Context) ([]specs.Platform, error) {
	return nil, r.permanentErr
}
func (r runtimeFwdErr) ResolveContainers(context.Context, *schema.Server) ([]*ContainerReference, error) {
	return nil, r.permanentErr
}
func (r runtimeFwdErr) NamespaceId() (*NamespaceId, error) {
	return nil, r.permanentErr
}
