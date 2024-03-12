// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ns

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/build/binary/genbinary"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/codegen"
	"namespacelabs.dev/foundation/internal/codegen/genpackage"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/internal/filewatcher"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/entity"
	integrationparsing "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/api"
	dockerfileparser "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/dockerfile"
	goparser "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/golang"
	nodejsparser "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/nodejs"
	shellparser "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/shellscript"
	webparser "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/web"
	"namespacelabs.dev/foundation/internal/git"
	"namespacelabs.dev/foundation/internal/integrations/golang"
	nodebinary "namespacelabs.dev/foundation/internal/integrations/nodejs/binary"
	nodeopaqueintegration "namespacelabs.dev/foundation/internal/integrations/nodejs/opaqueintegration"
	"namespacelabs.dev/foundation/internal/integrations/opaque"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/networking/ingress"
	"namespacelabs.dev/foundation/internal/networking/ingress/nginx"
	dockerfileapplier "namespacelabs.dev/foundation/internal/parsing/integration/dockerfile"
	goapplier "namespacelabs.dev/foundation/internal/parsing/integration/golang"
	nodejsapplier "namespacelabs.dev/foundation/internal/parsing/integration/nodejs"
	shellapplier "namespacelabs.dev/foundation/internal/parsing/integration/shellscript"
	webapplier "namespacelabs.dev/foundation/internal/parsing/integration/web"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/planning/tool"
	"namespacelabs.dev/foundation/internal/providers/nscloud"
	"namespacelabs.dev/foundation/internal/providers/nscloud/nsingress"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/helm"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/kubeops"
	"namespacelabs.dev/foundation/internal/sdk/gcloud"
	"namespacelabs.dev/foundation/internal/sdk/k3d"
	"namespacelabs.dev/foundation/internal/testing"
	"namespacelabs.dev/foundation/orchestration"
	"namespacelabs.dev/foundation/orchestration/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/module"
	"namespacelabs.dev/foundation/std/plugandplay"
	"namespacelabs.dev/foundation/universe/aws/iam"
)

func DoMain(name string, autoUpdate bool, registerCommands func(*cobra.Command)) {
	var deprecatedToolsInvocation bool

	fncobra.DoMain(fncobra.MainOpts{
		Name:       name,
		AutoUpdate: autoUpdate,
		RegisterCommands: func(rootCmd *cobra.Command) {
			registerCommands(rootCmd)

			fncobra.PushPreParse(rootCmd, func(ctx context.Context, args []string) error {
				module.WireModuleLoader()
				filewatcher.SetupFileWatcher()

				binary.BuildGo = golang.GoBuilder
				binary.BuildLLBGen = genbinary.LLBBinary
				binary.BuildAlpine = genbinary.BuildAlpine
				binary.BuildNix = genbinary.NixImageBuilder
				binary.BuildNodejs = nodebinary.NodejsBuilder
				binary.BuildStaticFilesServer = genbinary.StaticFilesServerBuilder

				// Runtime
				tool.RegisterInjection("schema.ComputedNaming", func(ctx context.Context, env cfg.Context, planner runtime.Planner, s *schema.Stack_Entry) (*schema.ComputedNaming, error) {
					n, err := runtime.ComputeNaming(ctx, env.Workspace().ModuleName(), env, planner, s.ServerNaming)
					if n == nil && err == nil {
						return &schema.ComputedNaming{}, nil // these type of injections can't return nil.
					}
					return n, err
				})

				deploy.RegisterDeployOps()

				// Compute cacheables.
				oci.RegisterImageCacheable()

				// Languages.
				golang.Register()
				opaque.Register()
				nodeopaqueintegration.Register()

				// Opaque integrations: parsing
				integrationparsing.IntegrationParser = entity.NewDispatchingEntityParser("kind", []entity.EntityParser{
					&dockerfileparser.Parser{},
					shellparser.NewParser(),
					goparser.NewParser(),
					&nodejsparser.Parser{},
					&webparser.Parser{},
				})
				integrationparsing.BuildParser = entity.NewDispatchingEntityParser("kind", []entity.EntityParser{
					// Same syntax as docker integration so we can reuse the parser.
					&dockerfileparser.Parser{},
					shellparser.NewParser(),
					// Same syntax as go integration so we can reuse the parser.
					goparser.NewParser(),
				})

				// Opaque integrations: applying
				dockerfileapplier.Register()
				goapplier.Register()
				nodejsapplier.Register()
				webapplier.Register()
				shellapplier.Register()

				// Codegen
				genpackage.Register()
				codegen.RegisterGraphHandlers()

				// Providers.
				plugandplay.RegisterProviders()
				iam.RegisterGraphHandlers()
				ingress.RegisterIngressClass(nginx.IngressClass())
				ingress.RegisterIngressClass(nsingress.IngressClass())

				// Runtimes.
				kubernetes.Register()
				kubeops.Register()
				helm.Register()
				orchestration.RegisterPrepare()

				cfg.Seal()

				if deprecatedToolsInvocation {
					fmt.Fprintf(console.Warnings(ctx), "Flag without effect: --tools_invocation_can_use_buildkit is now the default.\n")
				}

				return nil
			})

			nscloud.SetupFlags(rootCmd.PersistentFlags(), true)

			rootCmd.PersistentFlags().BoolVar(&binary.UsePrebuilts, "use_prebuilts", binary.UsePrebuilts,
				"If set to false, binaries are built from source rather than a corresponding prebuilt being used.")
			rootCmd.PersistentFlags().BoolVar(&compute.CachingEnabled, "caching", compute.CachingEnabled,
				"If set to false, compute caching is disabled.")
			rootCmd.PersistentFlags().BoolVar(&git.AssumeSSHAuth, "git_ssh_auth", !environment.IsRunningInCI(),
				"If set to true, assume that you use SSH authentication with git (this enables us to properly instruct git when downloading private repositories).")

			rootCmd.PersistentFlags().Var(buildkit.ImportCacheVar, "buildkit_import_cache", "Internal, set buildkit import-cache.")
			rootCmd.PersistentFlags().Var(buildkit.ExportCacheVar, "buildkit_export_cache", "Internal, set buildkit export-cache.")
			rootCmd.PersistentFlags().StringVar(&buildkit.BuildkitSecrets, "buildkit_secrets", "", "A list of secrets to pass in to buildkit.")
			rootCmd.PersistentFlags().BoolVar(&buildkit.ForwardKeychain, "buildkit_forward_keychain", buildkit.ForwardKeychain, "If set to true, proxy buildkit auth through namespace orchestration.")
			rootCmd.PersistentFlags().BoolVar(&compute.VerifyCaching, "verify_compute_caching", compute.VerifyCaching,
				"Internal, do not use cached contents of compute graph, verify that the cached content matches instead.")
			rootCmd.PersistentFlags().StringVar(&llbutil.GitCredentialsBuildkitSecret, "golang_buildkit_git_credentials_secret", "",
				"If set, go invocations in buildkit get the specified secret mounted as ~/.git-credentials")
			rootCmd.PersistentFlags().BoolVar(&deploy.AlsoDeployIngress, "also_compute_ingress", deploy.AlsoDeployIngress,
				"[development] Set to false, to skip ingress computation.")
			rootCmd.PersistentFlags().BoolVar(&buildkit.SkipExpectedMaxWorkspaceSizeCheck, "skip_buildkit_workspace_size_check", buildkit.SkipExpectedMaxWorkspaceSizeCheck,
				"If set to true, skips our enforcement of the maximum workspace size we're willing to push to buildkit.")
			rootCmd.PersistentFlags().BoolVar(&buildkit.ForceEstargz, "buildkit_force_estargz", buildkit.ForceEstargz,
				"If set to true, images are exported using estargz.")
			rootCmd.PersistentFlags().BoolVar(&k3d.IgnoreZfsCheck, "ignore_zfs_check", k3d.IgnoreZfsCheck,
				"If set to true, ignores checking whether the base system is ZFS based.")
			rootCmd.PersistentFlags().BoolVar(&deploy.RunCodegen, "run_codegen", deploy.RunCodegen, "If set to false, skip codegen.")
			rootCmd.PersistentFlags().BoolVar(&deploy.PushPrebuiltsToRegistry, "deploy_push_prebuilts_to_registry", deploy.PushPrebuiltsToRegistry,
				"If set to true, prebuilts are uploaded to the target registry.")
			rootCmd.PersistentFlags().BoolVar(&oci.ConvertImagesToEstargz, "oci_convert_images_to_estargz", oci.ConvertImagesToEstargz,
				"If set to true, images are converted to estargz before being uploaded to a registry.")
			rootCmd.PersistentFlags().BoolVar(&tool.InvocationDebug, "invocation_debug", tool.InvocationDebug,
				"If set to true, pass --debug to invocations.")
			rootCmd.PersistentFlags().BoolVar(&kubernetes.UseNodePlatformsForProduction, "kubernetes_use_node_platforms_in_production_builds",
				kubernetes.UseNodePlatformsForProduction, "If set to true, queries the target node platforms to determine what platforms to build for.")
			rootCmd.PersistentFlags().StringSliceVar(&kubernetes.ProductionPlatforms, "production_platforms", kubernetes.ProductionPlatforms,
				"The set of platforms that we build production images for.")
			rootCmd.PersistentFlags().BoolVar(&fnapi.NamingForceStored, "fnapi_naming_force_stored",
				fnapi.NamingForceStored, "If set to true, if there's a stored certificate, use it without checking the server.")
			rootCmd.PersistentFlags().BoolVar(&kubernetes.DeployAsPodsInTests, "kubernetes_deploy_as_pods_in_tests", kubernetes.DeployAsPodsInTests,
				"If true, servers in tests are deployed as pods instead of deployments.")
			rootCmd.PersistentFlags().BoolVar(&compute.ExplainIndentValues, "compute_explain_indent_values", compute.ExplainIndentValues,
				"If true, values output by --explain are indented.")
			rootCmd.PersistentFlags().BoolVar(&deprecatedToolsInvocation, "tools_invocation_can_use_buildkit", false,
				"If set to true, tool invocations will use buildkit whenever possible.")
			rootCmd.PersistentFlags().BoolVar(&testing.UseNamespaceCloud, "testing_use_namespace_cloud", testing.UseNamespaceCloud,
				"If set to true, allocate cluster for tests on demand.")
			rootCmd.PersistentFlags().BoolVar(&testing.UseNamespaceBuildCluster, "testing_use_namespace_cloud_build", testing.UseNamespaceBuildCluster,
				"If set to true, allocate a build cluster for tests.")
			rootCmd.PersistentFlags().BoolVar(&runtime.WorkInProgressUseShortAlias, "runtime_wip_use_short_alias", runtime.WorkInProgressUseShortAlias,
				"If set to true, uses the new ingress name allocator.")
			rootCmd.PersistentFlags().BoolVar(&client.UseOrchestrator, "use_orchestrator", client.UseOrchestrator,
				"If set to true, enables the incluster Namespace orchestrator (used for deployments, resource management and service readiness probing).")
			rootCmd.PersistentFlags().BoolVar(&orchestration.DeployWithOrchestrator, "deploy_with_orchestrator", orchestration.DeployWithOrchestrator,
				"If set to true, ns uses the incluster orchestrator for deployment.")
			rootCmd.PersistentFlags().BoolVar(&orchestration.RenderOrchestratorDeployment, "render_orchestrator_deployment", orchestration.RenderOrchestratorDeployment,
				"If set to true, we print a render wait block while deploying the orchestrator itself.")
			rootCmd.PersistentFlags().StringVar(&orchestration.SlackToken, "slack_token", "",
				"Token used to call Slack.")
			rootCmd.PersistentFlags().StringVar(&orchestration.DeployUpdateSlackChannel, "deploy_update_slack_channel", "",
				"Slack channel to send deployment notifications to.")
			rootCmd.PersistentFlags().BoolVar(&gcloud.UseHostGCloudBinary, "gcloud_use_host_binary", gcloud.UseHostGCloudBinary,
				"If set to true, uses a gcloud binary that is available at the host, rather than ns's builtin.")
			rootCmd.PersistentFlags().BoolVar(&filewatcher.FileWatcherUsePolling, "filewatcher_use_polling",
				filewatcher.FileWatcherUsePolling, "If set to true, uses polling to observe file system events.")
			rootCmd.PersistentFlags().BoolVar(&k3d.IgnoreVersionCheck, "k3d_ignore_docker_version", k3d.IgnoreVersionCheck,
				"If set to true, does not validate Docker's verison.")
			rootCmd.PersistentFlags().BoolVar(&kubeops.ForceApply, "kubernetes_force_apply", kubeops.ForceApply, "Whether to force-apply an Apply.")

			// We have too many flags, hide some of them from --help so users can focus on what's important.
			for _, noisy := range []string{
				"buildkit_import_cache",
				"buildkit_export_cache",
				"buildkit_secrets",
				"verify_compute_caching",
				"also_compute_ingress",
				"golang_buildkit_git_credentials_secret",
				"skip_buildkit_workspace_size_check",
				"ignore_zfs_check",
				"run_tools_on_kubernetes",
				"run_codegen",
				"invocation_debug",
				"kubernetes_use_node_platforms_in_production_builds",
				"production_platforms",
				"fnapi_naming_force_stored",
				"kubernetes_deploy_as_pods_in_tests",
				"compute_explain_indent_values",
				"nodejs_use_native_node",
				"lowlevel_tools_protocol_version",
				"tools_invocation_can_use_buildkit",
				"deploy_push_prebuilts_to_registry",
				"oci_convert_images_to_estargz",
				"runtime_wip_use_short_alias",
				"use_orchestrator",
				"deploy_with_orchestrator",
				"render_orchestrator_deployment",
				"buildkit_forward_keychain",
				"use_head_orchestrator",
				"update_orchestrator",
				"gcloud_use_host_binary",
				"filewatcher_use_polling",
				"k3d_ignore_docker_version",
				"kubernetes_force_apply",
				"slack_token",
				"slack_channel",
				// Hidden for M0
				"testing_use_namespace_cloud",
				"testing_use_namespace_cloud_build",
				"use_prebuilts",
			} {
				_ = rootCmd.PersistentFlags().MarkHidden(noisy)
			}
		},
	})
}
