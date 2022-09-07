// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console/termios"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/std/runtime"
)

const (
	FnServiceLivez  = "foundation.namespacelabs.dev/livez"
	FnServiceReadyz = "foundation.namespacelabs.dev/readyz"
)

type Runtime interface {
	// DeployedConfigImageID retrieves the image reference of the "configuration
	// image" used to deploy the specified server. Configuration images are only
	// generated for production environments for now.
	DeployedConfigImageID(context.Context, *schema.Server) (oci.ImageID, error)

	// Plans a deployment, i.e. produces a series of instructions that will
	// instantiate the required deployment resources to run the servers in the
	// specified Deployment. This method is side-effect free; mutations are
	// applied when the generated plan is applied.
	PlanDeployment(context.Context, Deployment) (DeploymentState, error)

	// Plans an ingress deployment, i.e. produces a series of instructions that
	// will instantiate the required deployment resources to run the servers in
	// the specified Ingresses. This method is side-effect free; mutations are
	// applied when the generated plan is applied.
	PlanIngress(context.Context, *schema.Stack, []*schema.IngressFragment) (DeploymentState, error)

	// ComputeBaseNaming returns a base naming configuration that is specific
	// to the target runtime (e.g. kubernetes cluster).
	ComputeBaseNaming(context.Context, *schema.Naming) (*schema.ComputedNaming, error)

	// Returns a list of containers that the server has deployed.
	ResolveContainers(context.Context, *schema.Server) ([]*ContainerReference, error)

	// Fetch logs of a specific container reference.
	FetchLogsTo(context.Context, io.Writer, *ContainerReference, FetchLogsOpts) error

	// Fetch diagnostics of a particular container reference.
	FetchDiagnostics(context.Context, *ContainerReference) (*Diagnostics, error)

	// Fetch environment diagnostics, e.g. event list.
	FetchEnvironmentDiagnostics(context.Context) (*storage.EnvironmentDiagnostics, error)

	// Starts a new shell in the container of a previously deployed server. The
	// image of the server must contain the specified command. For ephemeral
	// containers, see #329.
	StartTerminal(ctx context.Context, server *schema.Server, io TerminalIO, command string, rest ...string) error

	// Attaches to a previously running container.
	AttachTerminal(ctx context.Context, container *ContainerReference, io TerminalIO) error

	// Forwards a single port.
	ForwardPort(ctx context.Context, server *schema.Server, containerPort int32, localAddrs []string, callback SinglePortForwardedFunc) (io.Closer, error)

	// Dials a TCP port to one of the replicas of the target server. The
	// lifecycle of the connection is bound to the specified context.
	DialServer(ctx context.Context, server *schema.Server, containerPort int32) (net.Conn, error)

	// Exposes the cluster's ingress, in the specified local address and port.
	// This is used to create stable localhost-bound ingress addresses (for e.g.
	// nslocal.host).
	ForwardIngress(ctx context.Context, localAddrs []string, localPort int, f PortForwardedFunc) (io.Closer, error)

	// Observes lifecyle events of the specified server. Unless OneShot is set,
	// Observe runs until the context is cancelled.
	Observe(context.Context, *schema.Server, ObserveOpts, func(ObserveEvent) error) error

	// Runs the specified container as a one-shot, streaming it's output to the
	// specified writer. This mechanism is targeted at invoking test runners
	// within the runtime environment.
	RunOneShot(context.Context, string /*name*/, ServerRunOpts, io.Writer, bool /*follow*/) error

	// RunAttached runs the specified container, and attaches to it.
	RunAttached(context.Context, string, ServerRunOpts, TerminalIO) error

	// Deletes the scoped environment, and all of its associated resources (e.g.
	// after a test invocation). If wait is true, waits until the target
	// resources have been removed. Returns true if resources were deleted.
	DeleteRecursively(ctx context.Context, wait bool) (bool, error)

	// Deletes any runtime resource deployed by this runtime, regardless of
	// environment. If wait is true, waits until the target resources have been
	// removed. Returns true if resources were deleted.
	DeleteAllRecursively(ctx context.Context, wait bool, progress io.Writer) (bool, error)

	// Returns a human readable ID of the deployment namespace.
	// Different IDs signify that deployments are independent and can be executed in parallel.
	NamespaceId() (*NamespaceId, error)

	HasPrepareProvision
	HasTargetPlatforms
}

type DeferredRuntime interface {
	New(context.Context, planning.Context) (Runtime, error)
}

type HasPrepareProvision interface {
	// PrepareProvision is called before invoking a provisioning tool, to offer
	// the runtime implementation a way to pass runtime-specific information to
	// the tool. E.g. what's the Kubernetes namespace we're working with.
	PrepareProvision(context.Context, planning.Context) (*rtypes.ProvisionProps, error)
}

type HasTargetPlatforms interface {
	// Returns the set of platforms that the target runtime operates on, e.g. linux/amd64.
	TargetPlatforms(context.Context) ([]specs.Platform, error)
}

type Deployment struct {
	Focus   schema.PackageList
	Stack   *schema.Stack
	Servers []ServerConfig
	Secrets GroundedSecrets
}

type ServerConfig struct {
	ServerRunOpts
	ServerLocation   fnerrors.Location
	Server           *schema.Server
	ConfigImage      *oci.ImageID
	ServerExtensions []*schema.ServerExtension
	Extensions       []*schema.DefExtension
	Sidecars         []SidecarRunOpts
	Inits            []SidecarRunOpts
	RuntimeConfig    *runtime.RuntimeConfig
}

type GroundedSecrets struct {
	Secrets []GroundedSecret
}

type GroundedSecret struct {
	Owner schema.PackageName
	Name  string
	Value *schema.FileContents
}

type ServerRunOpts struct {
	WorkingDir         string
	Image              oci.ImageID
	Command            []string
	Args               []string
	Env                []*schema.BinaryConfig_EnvEntry
	RunAs              *RunAs
	ReadOnlyFilesystem bool
}

type RunAs struct {
	UserID  string
	FSGroup *string
}

type SidecarRunOpts struct {
	Name       string
	PackageRef *schema.PackageRef
	ServerRunOpts
}

type FetchLogsOpts struct {
	TailLines         int // Only used if it's a positive value.
	Follow            bool
	FetchLastFailure  bool
	IncludeTimestamps bool
}

type ObserveOpts struct {
	OneShot bool
}

type ObserveEvent struct {
	ContainerReference *ContainerReference
	HumanReadableID    string
	Added              bool
	Removed            bool
}

type DeploymentState interface {
	Definitions() []*schema.SerializedInvocation
	Hints() []string
}

type TerminalIO struct {
	TTY bool

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// Optional. If set, a runtime can listen on this channel for terminal resize requests.
	ResizeQueue chan termios.WinSize
}

type ForwardedPortEvent struct {
	Endpoint *schema.Endpoint

	Added   []ForwardedPort
	Removed bool
	Error   error
}

type ForwardedPort struct {
	LocalPort     uint
	ContainerPort uint
}

type PortForwardedFunc func(ForwardedPortEvent)
type SinglePortForwardedFunc func(ForwardedPort)

type EndpointPortForwarder interface {
	io.Closer
	Update([]*schema.Endpoint)
}

type ErrContainerExitStatus struct {
	ExitCode int32
}

func (e ErrContainerExitStatus) Error() string {
	return fmt.Sprintf("container exited with code %d", e.ExitCode)
}

type ErrContainerFailed struct {
	Name   string
	Reason string

	FailedContainers []*ContainerReference // A pointer that can be passed to the runtime to fetch logs.
}

func (e ErrContainerFailed) Error() string {
	return fmt.Sprintf("%s: container failed with: %s", e.Name, e.Reason)
}

type PortRange struct {
	Base, Max int32
}

func DefaultPortRange() PortRange { return PortRange{40000, 41000} }

func (cw *ContainerWaitStatus) WaitStatus() string {
	var inits []string
	for _, init := range cw.Initializers {
		inits = append(inits, fmt.Sprintf("%s: %s", init.Name, init.StatusLabel))
	}

	joinedInits := strings.Join(inits, "; ")

	switch len(cw.Containers) {
	case 0:
		return joinedInits
	case 1:
		return box(cw.Containers[0].StatusLabel, joinedInits)
	default:
		var labels []string
		for _, ctr := range cw.Containers {
			labels = append(labels, fmt.Sprintf("%s: %s", ctr.Name, ctr.StatusLabel))
		}

		return box(fmt.Sprintf("{%s}", strings.Join(labels, "; ")), joinedInits)
	}
}

func box(a, b string) string {
	if b == "" {
		return a
	}

	return fmt.Sprintf("%s [%s]", a, b)
}

func (d *Diagnostics) Failed() bool {
	return d.Terminated && d.ExitCode > 0
}

func (g GroundedSecrets) Get(owner, name string) *schema.FileContents {
	for _, secret := range g.Secrets {
		if secret.Owner.Equals(owner) && secret.Name == name {
			return secret.Value
		}
	}

	return nil
}
