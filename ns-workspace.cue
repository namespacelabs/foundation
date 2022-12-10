module: "namespacelabs.dev/foundation"
requirements: {
	api:          47
	toolsVersion: 4
}
prebuilts: {
	digest: {
		"namespacelabs.dev/foundation/internal/sdk/buf/image/prebuilt":                     "sha256:5a9f9711fcd93aa2cdb5d2ee2aaa1b2fdd23b7e139ff4a39438153668b9b84ef"
		"namespacelabs.dev/foundation/std/development/filesync/controller":                 "sha256:41ffa681aec6a70dcd5a7ebeccd94814688389a45f39810138a4d3f1ef8278da"
		"namespacelabs.dev/foundation/std/monitoring/grafana/tool":                         "sha256:346a38e8301ba8366659280249a16ec287a14559f2855f5e7f2d07e5e4c190f9"
		"namespacelabs.dev/foundation/std/monitoring/prometheus/tool":                      "sha256:067f86f8231c4787fa49d70251dba1c3b25d98bcfa020d21529994896786b5eb"
		"namespacelabs.dev/foundation/std/networking/gateway/controller":                   "sha256:11ff24b7079bd83001568570ccfac7b6118baa84f585901d54419bb7f08727a5"
		"namespacelabs.dev/foundation/std/networking/gateway/server/configure":             "sha256:a6b6fcb1f42e730004aa0fdf339130dea9665df1a2581f517b78137bbb3631c7"
		"namespacelabs.dev/foundation/std/runtime/kubernetes/kube-state-metrics/configure": "sha256:159e5af8e9c2724a272f1ff22a4d1b8d9e4f93e75fc8ac9b85309e36b6c8f676"
		"namespacelabs.dev/foundation/std/startup/testdriver":                              "sha256:39531c5b96518cee0a26037cb1ec7984a849d2f0a144ebf58c990832bdb5c9b0"
		"namespacelabs.dev/foundation/std/web/http/configure":                              "sha256:128c028ef235bc9a2a2cd3ecce42298a4414b29acbddf1755f1f1c0014a927f5"
	}
	baseRepository: "us-docker.pkg.dev/foundation-344819/prebuilts/"
}
internalAliases: [{
	module_name: "library.namespace.so"
	rel_path:    "library"
}]
