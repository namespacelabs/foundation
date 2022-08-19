module: "namespacelabs.dev/foundation"
requirements: {
	api:          37
	toolsVersion: 4
}
prebuilts: {
	digest: {
		"namespacelabs.dev/foundation/cmd/ns":                                                         "sha256:e5e53156c07c0a11f795c8eef1bf32b2fbd64a0882c1322a7b96656823f5c939"
		"namespacelabs.dev/foundation/cmd/nspipelines":                                                "sha256:ba6dc84bf1aad3f1bb88cc441c799b75417438485093eafa44cb5678bd347f4c"
		"namespacelabs.dev/foundation/devworkflow/web":                                                "sha256:7c23463fd307825ab082152a527e4863773e3d513699afd8affb1868aa0172c4"
		"namespacelabs.dev/foundation/internal/sdk/buf/image/prebuilt":                                "sha256:f2cf5502f9b6afc27f73386fbbfd31954434968b048723f37e22996228972ee3"
		"namespacelabs.dev/foundation/std/dev/controller":                                             "sha256:bff9b0ed48a0cd129ad9015882fd130ae268b51b3dadf211467b0f84bea430cb"
		"namespacelabs.dev/foundation/std/development/filesync/controller":                            "sha256:0bdc9f7dff58db8626c8cbb5cbac1d7aee3c93c4f1268d48ac667041ddb3c85b"
		"namespacelabs.dev/foundation/std/grpc/httptranscoding/configure":                             "sha256:397666f83975f1909c864c3a4ec7bc59cab845aae0a074933f0dafefe26a39d4"
		"namespacelabs.dev/foundation/std/monitoring/grafana/tool":                                    "sha256:346a38e8301ba8366659280249a16ec287a14559f2855f5e7f2d07e5e4c190f9"
		"namespacelabs.dev/foundation/std/monitoring/prometheus/tool":                                 "sha256:067f86f8231c4787fa49d70251dba1c3b25d98bcfa020d21529994896786b5eb"
		"namespacelabs.dev/foundation/std/networking/gateway/controller":                              "sha256:83848c09288ad978f8c6870841532edfd87f530cacb768d32546070c77575f39"
		"namespacelabs.dev/foundation/std/networking/gateway/server/configure":                        "sha256:44689af861de98bc3fcb590fd9a8f2a39f9e3024a7481603f92e4a930fc70e86"
		"namespacelabs.dev/foundation/std/runtime/kubernetes/controller/img":                          "sha256:311f4a86aa798365557abdbdf8aa87f7bbe8ee251f37dbb98a49f4e65a804f36"
		"namespacelabs.dev/foundation/std/runtime/kubernetes/controller/tool":                         "sha256:cd63a3ac8e14fe142b856fa3108b33920c68ecdca92ec91ac708b39463caef02"
		"namespacelabs.dev/foundation/std/runtime/kubernetes/kube-state-metrics/configure":            "sha256:159e5af8e9c2724a272f1ff22a4d1b8d9e4f93e75fc8ac9b85309e36b6c8f676"
		"namespacelabs.dev/foundation/std/secrets/kubernetes":                                         "sha256:8df7ec718b19a4888435909ab2917c65fc19a021a4e45bfa163b4ce6d601a0f7"
		"namespacelabs.dev/foundation/std/startup/testdriver":                                         "sha256:87ed023cec48cade0a5cdb4a433cc7859863cb0c7cddab6fd913249a1ead5ad5"
		"namespacelabs.dev/foundation/std/testdata/datastore/keygen":                                  "sha256:5733814d051904c67ec5543d2f63de700c2d55d6561f21fa6a44e8d8609a91ac"
		"namespacelabs.dev/foundation/std/web/http/configure":                                         "sha256:128c028ef235bc9a2a2cd3ecce42298a4414b29acbddf1755f1f1c0014a927f5"
		"namespacelabs.dev/foundation/universe/aws/ecr/configure":                                     "sha256:bf54f3fdc5bc1128e29c0164809a4df12d60c0688ea3cf5da74786016a032457"
		"namespacelabs.dev/foundation/universe/aws/irsa/prepare":                                      "sha256:22f60c1f15911439a4711945245317acfa246184f94e2b7b5956131008c5dfe8"
		"namespacelabs.dev/foundation/universe/aws/s3/internal/configure":                             "sha256:0f2760d58ee3d4ec8aee1bd47d24d25cd730b888af047bf11ef21db570fff01d"
		"namespacelabs.dev/foundation/universe/aws/s3/internal/managebuckets/init":                    "sha256:40669c96749271e2f1247d98836d335949145415bda706c19bf6095a4a6df5f2"
		"namespacelabs.dev/foundation/universe/db/maria/incluster/tool":                               "sha256:252f83abd974d39c6ba258d21927dec1b514f893824f44ec7d7f0dc6e54e6b92"
		"namespacelabs.dev/foundation/universe/db/maria/internal/init":                                "sha256:1206bced820ab30286a5b3ad9baacbe1447e86e7aed4d2f2d2278fc0fa8a235a"
		"namespacelabs.dev/foundation/universe/db/maria/server/creds/tool":                            "sha256:0b0556ccca9e7e31d4e71779d5b9f4db7110f0b0f66593d0b0273f44b56e185e"
		"namespacelabs.dev/foundation/universe/db/maria/server/img":                                   "sha256:a7d5d37fe08eca6e91f88232784c92a6d411331a53aac7fcccb3b322875f9cb4"
		"namespacelabs.dev/foundation/universe/db/postgres/incluster/tool":                            "sha256:6495b69b8bea23dd7f7121229e15d0f785b2f9a92ec26fc2736c669cb474f089"
		"namespacelabs.dev/foundation/universe/db/postgres/internal/init":                             "sha256:d4b1e34b623bcb4e0ff535bab3725ff951f026b62b9f6a2016a78f279a195fed"
		"namespacelabs.dev/foundation/universe/db/postgres/opaque/tool":                               "sha256:42d380b51576ea211b7e9744dae5f0a960e20879a2da170cf7af538f15a24ede"
		"namespacelabs.dev/foundation/universe/db/postgres/rds/init":                                  "sha256:c4f0b3e2b4ccdcc13bc58093ed8ef3502357b59df5626a5a4acb2123d3ed61a4"
		"namespacelabs.dev/foundation/universe/db/postgres/rds/prepare":                               "sha256:54f1b2bddb6f0e01cde7bd53ee083ca3dcce32834cb5d86e8e36a0780b74d21b"
		"namespacelabs.dev/foundation/universe/db/postgres/server/creds/tool":                         "sha256:205d7b5200e5f606ac5654951a5f2db0e1b6ca4ec55dc795cb9d780c29474ba9"
		"namespacelabs.dev/foundation/universe/db/postgres/server/img":                                "sha256:73b5e1ad8011ac702d6232ac5d3e10cb05a5baccc6ea3e87bbd79008f140eca0"
		"namespacelabs.dev/foundation/universe/development/localstack/s3/internal/configure":          "sha256:b72a8f03cb49e98c0d7c105086502e193a039b61dd5b7d30b8a06fc5bec9e71f"
		"namespacelabs.dev/foundation/universe/development/localstack/s3/internal/managebuckets/init": "sha256:59a43cac29183cb5df7bd6e61e2fe9ea6a3a582181f690fad8c5323bd3408037"
		"namespacelabs.dev/foundation/universe/networking/k8s-event-exporter/configure":               "sha256:44409819476881e3ed2e962fb3a3214500250495fc41ddb286a9503613dc091a"
		"namespacelabs.dev/foundation/universe/networking/tailscale/image":                            "sha256:444639fe064c0be98ddf66671d93db47ba973ab17636254906b228d69d5b06a4"
		"namespacelabs.dev/foundation/universe/storage/s3/internal/managebuckets":                     "sha256:595779e09f0b3f614b9b022489f6a6d4b6c6ceec894e5273cfe69bb9aadbe347"
		"namespacelabs.dev/foundation/universe/storage/s3/internal/prepare":                           "sha256:56d14c7d02317dc9a1be83e97f29986e9a3dc738c674aa13208f41f4fdec567f"
	}
	baseRepository: "us-docker.pkg.dev/foundation-344819/prebuilts/"
}
