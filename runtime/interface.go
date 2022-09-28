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

// A runtime class represents a runtime implementation type, e.g. "kubernetes".
// The codebase seldom interacts with Class, but instead of Cluster instances
// obtained from a runtime class.
type Class interface {
	// Attaches to an existing cluster. Fails if the cluster doesn't exist or
	// the provider used would have instantiated a new cluster.
	AttachToCluster(context.Context, planning.Configuration) (Cluster, error)

	// Attaches to an existing cluster (if not is specified in the
	// configuration), or creates a new cluster as needed.
	EnsureCluster(context.Context, planning.Configuration) (Cluster, error)
}

// A cluster represents a cluster where Namespace is capable of deployment one
// or more applications.
type Cluster interface {
	// Returns a Planner implementation which emits deployment plans that target
	// a namespace within this cluster.
	Planner(planning.Context) Planner

	// Returns a namespace'd cluster -- one for a particular application use,
	// bound to the workspace identified by the planning.Context.
	Bind(planning.Context) (ClusterNamespace, error)

	// Fetch diagnostics of a particular container reference.
	FetchDiagnostics(context.Context, *ContainerReference) (*Diagnostics, error)

	// Fetch logs of a specific container reference.
	FetchLogsTo(ctx context.Context, destination io.Writer, container *ContainerReference, opts FetchLogsOpts) error

	// Attaches to a running container.
	AttachTerminal(ctx context.Context, container *ContainerReference, io TerminalIO) error

	// Exposes the cluster's ingress, in the specified local address and port.
	// This is used to create stable localhost-bound ingress addresses (for e.g.
	// nslocal.host).
	ForwardIngress(ctx context.Context, localAddrs []string, localPort int, notify PortForwardedFunc) (io.Closer, error)

	// EnsureState ensures that a cluster-specific bit of initialization is done once per instance.
	EnsureState(context.Context, string) (any, error)

	// Deletes any runtime resource deployed by this runtime, regardless of
	// environment. If wait is true, waits until the target resources have been
	// removed. Returns true if resources were deleted.
	DeleteAllRecursively(ctx context.Context, wait bool, progress io.Writer) (bool, error)
}

// A planner is capable of generating namespace-specific deployment plans. It
// may obtain external data in order to produce a plan, but none of its methods
// mutate outside state in order to do so.
type Planner interface {
	// Returns a representation of the Namespace this Planner will generate
	// plans to.
	Namespace() Namespace

	// Plans a deployment, i.e. produces a series of instructions that will
	// instantiate the required deployment resources to run the servers in the
	// specified Deployment. This method is side-effect free; mutations are
	// applied when the generated plan is applied.
	PlanDeployment(context.Context, DeploymentSpec) (*DeploymentPlan, error)

	// Plans an ingress deployment, i.e. produces a series of instructions that
	// will instantiate the required deployment resources to run the servers
	// with the specified Ingresses. This method is side-effect free; mutations
	// are applied when the generated plan is applied.
	PlanIngress(context.Context, *schema.Stack, []*schema.IngressFragment) (*DeploymentPlan, error)

	// PrepareProvision is called before invoking a provisioning tool, to offer
	// the runtime implementation a way to pass runtime-specific information to
	// the tool. E.g. what's the Kubernetes namespace we're working with.
	// XXX move to planner.
	PrepareProvision(context.Context) (*rtypes.ProvisionProps, error)

	// ComputeBaseNaming returns a base naming configuration that is specific
	// to the target runtime (e.g. kubernetes cluster).
	ComputeBaseNaming(*schema.Naming) (*schema.ComputedNaming, error)

	// Returns the set of platforms that the target runtime operates on, e.g. linux/amd64.
	TargetPlatforms(context.Context) ([]specs.Platform, error)
}

// Represents an application deployment target within a cluster. Clusters may
// provider one, or more co-existing Namespaces.
type Namespace interface {
	// XXX document guarantees.
	UniqueID() string
}

// ClusterNamespace represents a target deployment environment, scoped to an application
// (usually the combination of an environment and workspace).
type ClusterNamespace interface {
	// Returns a reference to the cluster where this namespace exists.
	Cluster() Cluster

	// Planner returns a Planner bound to the same namespace as this ClusterNamespace.
	Planner() Planner

	// DeployedConfigImageID retrieves the image reference of the "configuration
	// image" used to deploy the specified server. Configuration images are only
	// generated for production environments for now.
	DeployedConfigImageID(context.Context, Deployable) (oci.ImageID, error)

	// Returns a list of containers that the server has deployed.
	ResolveContainers(context.Context, Deployable) ([]*ContainerReference, error)

	// Fetch environment diagnostics, e.g. event list.
	FetchEnvironmentDiagnostics(context.Context) (*storage.EnvironmentDiagnostics, error)

	// Starts a new shell in the container of a previously deployed server. The
	// image of the server must contain the specified command. For ephemeral
	// containers, see #329.
	StartTerminal(ctx context.Context, server Deployable, io TerminalIO, command string, rest ...string) error

	// Forwards a single port.
	// XXX remove; callers should instead implement their own TCP net.Listener
	// and call DialServer as needed.
	ForwardPort(ctx context.Context, server Deployable, containerPort int32, localAddrs []string, notify SinglePortForwardedFunc) (io.Closer, error)

	// Dials a TCP port to one of the replicas of the target server. The
	// lifecycle of the connection is bound to the specified context.
	DialServer(ctx context.Context, server Deployable, containerPort int32) (net.Conn, error)

	// Observes lifecyle events of the specified server. Unless OneShot is set,
	// Observe runs until the context is cancelled.
	Observe(context.Context, Deployable, ObserveOpts, func(ObserveEvent) error) error

	// Waits until the specified deployable is no longer running (typically a one-shot).
	WaitForTermination(ctx context.Context, object Deployable) ([]ContainerStatus, error)

	// Deletes a previously deployed DeployableSpec.
	DeleteDeployment(ctx context.Context, deployable Deployable) error

	// Deletes the scoped environment, and all of its associated resources (e.g.
	// after a test invocation). If wait is true, waits until the target
	// resources have been removed. Returns true if resources were deleted.
	DeleteRecursively(ctx context.Context, wait bool) (bool, error)
}

type Deployable interface {
	// Returns a string to be compatible with the proto API.
	GetPackageName() string // schema.PackageName

	GetId() string

	GetName() string

	// Returns a string to be compatible with the proto API.
	GetDeployableClass() string // schema.DeployableClass
}

type DeploymentSpec struct {
	Specs   []DeployableSpec
	Secrets GroundedSecrets
}

type DeployableSpec struct {
	Location    fnerrors.Location
	PackageName schema.PackageName
	Focused     bool // Set to true if the user explicitly asked for this object to be deployed.
	Attachable  AttachableKind

	Class   schema.DeployableClass
	Id      string // Must not be empty.
	Name    string // Can be empty.
	Volumes []*schema.Volume

	MainContainer ContainerRunOpts
	Sidecars      []SidecarRunOpts
	Inits         []SidecarRunOpts

	ConfigImage   *oci.ImageID
	RuntimeConfig *runtime.RuntimeConfig

	Extensions []*schema.DefExtension

	Endpoints         []*schema.Endpoint         // Owned by this deployable.
	InternalEndpoints []*schema.InternalEndpoint // Owned by this deployable.
}

type AttachableKind string

const (
	AttachableKind_WITH_STDIN_ONLY AttachableKind = "stdin-only"
	AttachableKind_WITH_TTY        AttachableKind = "with-tty"
)

var _ Deployable = DeployableSpec{}

func (d DeployableSpec) GetId() string              { return d.Id }
func (d DeployableSpec) GetName() string            { return d.Name }
func (d DeployableSpec) GetDeployableClass() string { return string(d.Class) }
func (d DeployableSpec) GetPackageName() string     { return string(d.PackageName) }

type GroundedSecrets struct {
	Secrets []GroundedSecret
}

type GroundedSecret struct {
	Owner schema.PackageName
	Name  string
	Value *schema.FileContents
}

type ContainerRunOpts struct {
	WorkingDir         string
	Image              oci.ImageID
	Command            []string
	Args               []string
	Env                []*schema.BinaryConfig_EnvEntry
	RunAs              *RunAs
	ReadOnlyFilesystem bool
	Mounts             []*schema.Mount
}

type ContainerStatus struct {
	Reference        *ContainerReference
	TerminationError error
}

type RunAs struct {
	UserID  string
	FSGroup *string
}

type SidecarRunOpts struct {
	Name  string
	Owner *schema.PackageRef
	ContainerRunOpts
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

type DeploymentPlan struct {
	Definitions []*schema.SerializedInvocation
	Hints       []string
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
