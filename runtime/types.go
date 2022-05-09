// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console/termios"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
)

const (
	FnServiceLivez  = "foundation.namespacelabs.dev/livez"
	FnServiceReadyz = "foundation.namespacelabs.dev/readyz"
)

type Runtime interface {
	// PrepareProvision is called before invoking a provisioning tool, to offer the runtime
	// implementation a way to pass runtime-specific information to the tool. E.g. what's the
	// Kubernetes namespace we're working with.
	PrepareProvision(context.Context) (*rtypes.ProvisionProps, error)

	// DeployedConfigImageID retrieves the image reference of the "configuration image" used
	// to deploy the specified server. Configuration images are only generated for production
	// environments for now.
	DeployedConfigImageID(context.Context, *schema.Server) (oci.ImageID, error)

	// Plans a deployment, i.e. produces a series of instructions that will instantiate the
	// required deployment resources to run the servers in the specified Deployment. This
	// method is side-effect free; mutations are applied when the generated plan is applied.
	PlanDeployment(context.Context, Deployment) (DeploymentState, error)

	// Plans an ingress deployment, i.e. produces a series of instructions that will instantiate
	// the required deployment resources to run the servers in the specified Ingresses. This
	// method is side-effect free; mutations are applied when the generated plan is applied.
	PlanIngress(context.Context, *schema.Stack, []*schema.IngressFragment) (DeploymentState, error)

	// Plans a stack shutdown, i.e. produces a series of instructions that will delete the
	// previously added resources. This method is side-effect free; mutations are applied when
	// the generated plan is applied.
	PlanShutdown(ctx context.Context, focus []provision.Server, stack []provision.Server) ([]*schema.Definition, error)

	// Streams logs from a previously deployed server.
	StreamLogsTo(context.Context, io.Writer, *schema.Server, StreamLogsOpts) error

	// Fetch logs of a specific container reference.
	FetchLogsTo(context.Context, io.Writer, ContainerReference, FetchLogsOpts) error

	// Fetch diagnostics of a particular container reference.
	FetchDiagnostics(context.Context, ContainerReference) (Diagnostics, error)

	// Starts a new shell in the container of a previously deployed server. The image of the
	// server must contain the specified command. For ephemeral containers, see #329.
	StartTerminal(ctx context.Context, server *schema.Server, io TerminalIO, command string, rest ...string) error

	// Forwards a single port.
	ForwardPort(ctx context.Context, server *schema.Server, endpoint *schema.Endpoint, localAddrs []string, callback SinglePortForwardedFunc) (io.Closer, error)

	// Exposes the cluster's ingress, in the specified local address and port. This is used to
	// create stable localhost-bound ingress addresses (for e.g. nslocal.host).
	ForwardIngress(ctx context.Context, localAddrs []string, localPort int, f PortForwardedFunc) (io.Closer, error)

	// Creates a one-shot container, in the same isolation domain as other servers (in Kubernetes,
	// it would be the same namespace), with the specified image. This container is meant to be
	// used for debugging purposes.
	DebugShell(ctx context.Context, imageID oci.ImageID, io rtypes.IO) error

	// Observes lifecyle events of the specified server. Unless OneShot is set, Observe runs until the context is cancelled.
	Observe(context.Context, *schema.Server, ObserveOpts, func(ObserveEvent) error) error

	// Runs the specified container as a one-shot, streaming it's output to the specified writer.
	// This mechanism is targeted at invoking test runners within the runtime environment.
	RunOneShot(context.Context, schema.PackageName, ServerRunOpts, io.Writer) error

	// Deploys a controller with appropriate cluster permissions.
	RunController(context.Context, ServerRunOpts) error

	// Deletes the scoped environment, and all of its associated resources (e.g. after a test invocation).
	DeleteRecursively(context.Context) error

	// Returns the set of platforms that the target runtime operates on, e.g. linux/amd64.
	TargetPlatforms(context.Context) ([]specs.Platform, error)
}

type Deployment struct {
	BuildID provision.BuildID
	Focus   schema.PackageList
	Stack   *schema.Stack
	Servers []ServerConfig
}

type ServerConfig struct {
	ServerRunOpts
	Server      provision.Server
	ConfigImage *oci.ImageID
	Extensions  []*schema.DefExtension
	Sidecars    []SidecarRunOpts
	Inits       []SidecarRunOpts
}

type ServerRunOpts struct {
	WorkingDir         string
	Image              oci.ImageID
	Command            []string
	Args               []string
	Env                []*schema.BinaryConfig_Entry
	RunAs              *RunAs
	ReadOnlyFilesystem bool
}

type RunAs struct {
	UserID  string
	FSGroup *string
}

type SidecarRunOpts struct {
	PackageName schema.PackageName
	ServerRunOpts
}

type StreamLogsOpts struct {
	TailLines        int // Only used if it's a positive value.
	Follow           bool
	FetchLastFailure bool
}

type FetchLogsOpts struct {
	TailLines        int // Only used if it's a positive value.
	Follow           bool
	FetchLastFailure bool
}

type ObserveOpts struct {
	OneShot bool
}

type ObserveEvent struct {
	ContainerReference ContainerReference
	HumanReadableID    string
	Added              bool
	Removed            bool
}

type DeploymentState interface {
	Definitions() []*schema.Definition
	Hints() []string
}

type TerminalIO struct {
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

type ErrContainerFailedToStart struct {
	Name   string
	Reason string

	FailedContainers []ContainerReference // A pointer that can be passed to the runtime to fetch logs.
}

func (e ErrContainerFailedToStart) Error() string {
	return fmt.Sprintf("%s: container did not start successfully, failed with: %s", e.Name, e.Reason)
}

type ContainerWaitStatus struct {
	Containers   []ContainerUnitWaitStatus
	Initializers []ContainerUnitWaitStatus
}

type ContainerUnitWaitStatus struct {
	Reference   ContainerReference
	Name        string
	StatusLabel string

	Status Diagnostics
}

type ContainerReference interface {
	UniqueID() string
	HumanReference() string
}

func (cw ContainerWaitStatus) WaitStatus() string {
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

type Diagnostics struct {
	Running bool
	Started time.Time

	Waiting       bool
	WaitingReason string
	Crashed       bool

	Terminated       bool
	TerminatedReason string
	ExitCode         int32

	RestartCount int32
}

func (d Diagnostics) Failed() bool {
	return d.Terminated && d.ExitCode > 0
}
