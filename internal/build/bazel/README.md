# Building binary.cue images with rules_img

Use `bazel_build` to import a `rules_img` image into the normal Namespace binary
build, composition, and publishing flow:

```cue
binary: {
    name: "server"
    build_plan: [{bazel_build: {
        target: "//images:server"
        platforms: {
            "linux/amd64": "//platforms:linux_amd64"
            "linux/arm64": "//platforms:linux_arm64"
        }
    }}]
}
```

`target` is an absolute Bazel label, resolved from the Namespace module containing
the binary definition. That module root must also be a Bazel workspace. Bazel
runs on the machine running Namespace and uses the workspace's Bazel configuration,
including any configured remote execution or remote cache.

`platforms` maps each requested OCI platform to a Bazel `platform` target, passed
as `--platforms`. If supplied, it must cover every requested platform. Omit it
when the Bazel target already selects its platforms, for example an `image_index`
or an `image_manifest` with an explicit `platform` attribute. Namespace imports
only the requested platform and fails if it is absent or ambiguous; it never
relabels a host-platform image as the requested target platform.

The Bazel target must expose exactly one complete OCI layout tar archive in its
`oci_tarball` output group. Both `rules_img.image_manifest` and
`rules_img.image_index` provide this group. A minimal `images/BUILD.bazel` is:

```starlark
load("@rules_img//img:image.bzl", "image_manifest")
load("@rules_img//img:layer.bzl", "image_layer")

image_layer(
    name = "server_layer",
    srcs = {"/server": "//cmd/server:server"},
)

image_manifest(
    name = "server",
    layers = [":server_layer"],
    entrypoint = ["/server"],
)
```

Add `rules_img` to `MODULE.bazel` using its installation instructions, and define
the referenced binary and platform targets. No `image_push` or `image_load` target
is needed. Namespace requests `--output_groups=oci_tarball` and
`--remote_download_outputs=toplevel`, imports the resulting image, and uses its
existing publishing path. Do not enable rules_img push-at-build-time if builds
must remain separate from publishing.

The tar output avoids symlinked blobs in rules_img's directory layouts and lets
Namespace retain the image independently of subsequent Bazel builds. Both image
and index imports have been exercised with rules_img 0.3.23.

For pulled bases, configure `pull(layer_handling = "eager")` or the experimental
`"lazy"` mode. The default `"shallow"` mode does not supply the blobs required by
this integration. Sparse OCI layouts and non-materialized compact layers are not
supported. This integration downloads complete layer bytes rather than using
rules_img's direct remote-cache-to-registry push strategies.

`bazel_build` can be used anywhere another image plan can: as `from`, as one layer
of `build_plan`, or inside `files_from`. Existing composition semantics still
apply: plans build independently, their layers are appended in order, and the
first image supplies the base image configuration.
