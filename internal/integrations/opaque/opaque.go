// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package opaque

import (
	"context"
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/assets"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/workspace/wsremote"
	"namespacelabs.dev/foundation/internal/hotreload"
	"namespacelabs.dev/foundation/internal/integrations"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/runtime/constants"
)

func Register() {
	integrations.Register(schema.Framework_OPAQUE, OpaqueIntegration{})
}

type OpaqueIntegration struct {
	integrations.MaybeGenerate
	integrations.MaybeTidy // TODO implement tidy per parser.
}

func (OpaqueIntegration) PrepareBuild(ctx context.Context, assets assets.AvailableBuildAssets, server planning.Server, isFocus bool) (build.Spec, error) {
	binRef := server.Proto().GetMainContainer().GetBinaryRef()

	if binRef == nil {
		return nil, fnerrors.InternalError("server binary is not set at %s", server.Location)
	}

	pkg, err := server.SealedContext().LoadByName(ctx, binRef.AsPackageName())
	if err != nil {
		return nil, err
	}

	prep, err := binary.Plan(ctx, pkg, binRef.GetName(), server.SealedContext(), assets,
		binary.BuildImageOpts{UsePrebuilts: true})
	if err != nil {
		return nil, err
	}

	filesyncConfig, err := getFilesyncWorkspacePath(server)
	if err != nil {
		return nil, err
	}

	if filesyncConfig != nil {
		pkg, err := server.SealedContext().LoadByName(ctx, hotreload.ControllerPkg.AsPackageName())
		if err != nil {
			return nil, err
		}

		ctrlBin, err := binary.Plan(ctx, pkg, hotreload.ControllerPkg.Name, server.SealedContext(), assets, binary.BuildImageOpts{UsePrebuilts: false})
		if err != nil {
			return nil, err
		}

		return binary.MergeSpecs{
			Specs:        []build.Spec{prep.Plan.Spec, ctrlBin.Plan.Spec},
			Descriptions: []string{prep.Name, "workspace sync controller"},
		}, nil
	} else {
		return prep.Plan.Spec, nil
	}
}

func (OpaqueIntegration) PrepareRun(ctx context.Context, server planning.Server, run *runtime.ContainerRunOpts) error {
	binRef := server.Proto().GetMainContainer().GetBinaryRef()
	if binRef != nil {
		_, binary, err := pkggraph.LoadBinary(ctx, server.SealedContext(), binRef)
		if err != nil {
			return err
		}

		config := binary.Config
		if config != nil {
			run.WorkingDir = config.WorkingDir
			run.Command = config.Command
			run.Args = config.Args
			run.Env = config.Env
		}

		filesyncConfig, err := getFilesyncWorkspacePath(server)
		if err != nil {
			return err
		}

		if filesyncConfig != nil {
			if len(run.Command) == 0 {
				return fnerrors.NewWithLocation(server.Location, "dockerfile command must be explicitly set when there is a workspace sync mount")
			}

			run.Args = append(append(
				[]string{filesyncConfig.mountPath, fmt.Sprint(hotreload.FileSyncPort)},
				run.Command...),
				run.Args...)
			run.Command = []string{hotreload.ControllerCommand}
		}
	}

	return nil
}

func (OpaqueIntegration) PrepareDev(ctx context.Context, cluster runtime.ClusterNamespace, server planning.Server) (context.Context, integrations.DevObserver, error) {
	filesyncConfig, err := getFilesyncWorkspacePath(server)
	if err != nil {
		return nil, nil, err
	}

	if filesyncConfig != nil {
		return hotreload.ConfigureFileSyncDevObserver(ctx, cluster, server)
	}

	return ctx, nil, nil
}

func (OpaqueIntegration) PreParseServer(ctx context.Context, loc pkggraph.Location, ext *parsing.ServerFrameworkExt) error {
	return nil
}

func (OpaqueIntegration) PostParseServer(ctx context.Context, _ *parsing.Sealed) error {
	return nil
}

func (OpaqueIntegration) DevelopmentPackages() []schema.PackageName {
	return nil
}

func (OpaqueIntegration) PrepareHotReload(ctx context.Context, remote *wsremote.SinkRegistrar, srv planning.Server) *integrations.HotReloadOpts {
	if remote == nil {
		return nil
	}

	filesyncConfig, err := getFilesyncWorkspacePath(srv)
	if err != nil {
		// Shouldn't happen because getFilesyncWorkspacePath() is already called in PrepareDev().
		panic(fnerrors.InternalError("Error from getFilesyncWorkspacePath in PrepareHotReload, shouldn't happen: %v", err))
	}

	if filesyncConfig == nil {
		return nil
	}

	return &integrations.HotReloadOpts{
		// "ModuleName" and "Rel" are empty because we have only one module in the image and
		// we put the package content directly under the root "/app" directory.
		Sink: remote.For(&wsremote.Signature{ModuleName: "", Rel: ""}),
		EventProcessor: func(ev *wscontents.FileEvent) *wscontents.FileEvent {
			if strings.HasPrefix(ev.Path, filesyncConfig.srcPath+"/") {
				return &wscontents.FileEvent{
					Event:       ev.Event,
					Path:        ev.Path[len(filesyncConfig.srcPath)+1:],
					NewContents: ev.NewContents,
					Mode:        ev.Mode,
				}
			} else {
				return nil
			}
		},
	}
}

type filesyncConfig struct {
	// Relative to the package
	srcPath   string
	mountPath string
}

// If not nil, filesync is requested and enabled.
func getFilesyncWorkspacePath(server planning.Server) (*filesyncConfig, error) {
	if !UseDevBuild(server.SealedContext().Environment()) {
		return nil, nil
	}

	for _, m := range server.Proto().MainContainer.Mount {
		// Only supporting volumes within the same package for now.
		v, err := findVolume(server.Proto().Volume, m.VolumeRef.Name)
		if err != nil {
			return nil, err
		}

		if v.Kind == constants.VolumeKindWorkspaceSync {
			cv := &schema.WorkspaceSyncVolume{}
			if err := v.Definition.UnmarshalTo(cv); err != nil {
				return nil, fnerrors.InternalError("%s: failed to unmarshal workspacesync volume definition: %w", v.Name, err)
			}

			return &filesyncConfig{cv.Path, m.Path}, nil
		}
	}

	return nil, nil
}

func findVolume(volumes []*schema.Volume, name string) (*schema.Volume, error) {
	for _, v := range volumes {
		if v.Name == name {
			return v, nil
		}
	}
	return nil, fnerrors.InternalError("volume %s not found", name)
}
