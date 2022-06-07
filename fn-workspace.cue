module: "namespacelabs.dev/foundation"
requirements: {
	api: 33
}

prebuilts: {
	digest: {
		"namespacelabs.dev/foundation/devworkflow/web":                                                "sha256:0d52c0b3b6d6bce69044ab12c66e40c08c58b63ab9dcdf445df9dacd21e07ba6"
		"namespacelabs.dev/foundation/std/dev/controller":                                             "sha256:7c1afe44f257c7f957216065186f6c291007d1b0d6c94487aeae928a4b1e6609"
		"namespacelabs.dev/foundation/std/development/filesync/controller":                            "sha256:6d311d86a96646b5659c573d7ec2d6e4f5e630bec00e087a45fcb0f99994d149"
		"namespacelabs.dev/foundation/std/monitoring/grafana/tool":                                    "sha256:b4a6a8c73f2a6b7864d5c81236accc743924ec6c519a9b515b1132b6ada2374e"
		"namespacelabs.dev/foundation/std/monitoring/prometheus/tool":                                 "sha256:9166f292c1d54da2f3e4cae754a5f6a6f58f5fe894e6fa71d37daa0460049e45"
		"namespacelabs.dev/foundation/std/runtime/kubernetes/controller/img":                          "sha256:82079659cffbc02d5b110790a0d0001ad1e255e70f326a843d14d63a225dd1c5"
		"namespacelabs.dev/foundation/std/runtime/kubernetes/controller/tool":                         "sha256:714d8466d5cc5e13e28ce05a8e3e46b2bd8bae4afb2e662b7bfa75ec965a290d"
		"namespacelabs.dev/foundation/std/sdk/buf/baseimg":                                            "sha256:df46f86a84d091f60182a862f9f850bb15b2aa7d030d3abba375d56fafc5168f"
		"namespacelabs.dev/foundation/std/sdk/buf/baseimgnix":                                         "sha256:0f4289144c4d712308b6082cae42474be2be638dbb9445e39248153caa1576cb"
		"namespacelabs.dev/foundation/std/secrets/kubernetes":                                         "sha256:e18d7b0b58fa574080a2265016d9d238646b94475e3939a06aeadc71d01847fe"
		"namespacelabs.dev/foundation/std/startup/testdriver":                                         "sha256:7ea188a4a7d5ea59a127f2bdcc56189d5a31af9f8d9e6d7ee242ae77c37adf5a"
		"namespacelabs.dev/foundation/std/testdata/datastore/keygen":                                  "sha256:3af31c69e50e156eb208afde803a3fd1deeae3cb7f01fb4ffb7076e80beb85b3"
		"namespacelabs.dev/foundation/universe/aws/irsa/prepare":                                      "sha256:44bb7a8352e292f9717e467b4cf0138b4baeb5bc8936a65069e20672ce03ca18"
		"namespacelabs.dev/foundation/universe/aws/s3/internal/configure":                             "sha256:9d60f157d7ec2a36577552e899ee94f998a264054393ad609a5eb676e648877b"
		"namespacelabs.dev/foundation/universe/aws/s3/internal/managebuckets/init":                    "sha256:921fe3758b2720d4e6d76777892c7a55dbed7b17d70d5eceb631464d22fd5dce"
		"namespacelabs.dev/foundation/universe/db/maria/incluster/tool":                               "sha256:1b80f395f249cd373674971590af59f106edbb6ad5f9ec40ce0ce7ef948972c9"
		"namespacelabs.dev/foundation/universe/db/maria/init":                                         "sha256:68f82d6ddf094ad9ff03c93bb0e4d222f2ad0c19e29b61f686e3c055e50a7993"
		"namespacelabs.dev/foundation/universe/db/maria/server/creds/tool":                            "sha256:816654523894cca56e40f7d5f81587b2f2d91db20169b0cc0e51ce29996deeaa"
		"namespacelabs.dev/foundation/universe/db/postgres/incluster/tool":                            "sha256:2d872f7c1a71fdb6e8769d850d405602827695b96c0e974a0bfc5e48f1d0ea6b"
		"namespacelabs.dev/foundation/universe/db/postgres/init":                                      "sha256:9770663f2e3aa19e88930475af8d4274934177d2042c0f2cb65c412ed4bd1d95"
		"namespacelabs.dev/foundation/universe/db/postgres/opaque/tool":                               "sha256:b61d456c5d9f33c3faa71fad2451c913923d25fa07f15986ebaac92c573a4773"
		"namespacelabs.dev/foundation/universe/db/postgres/server/creds/tool":                         "sha256:262c4069dd79d2659dc40d10024be6c525d5a510980ed2fced3a26778977c2e0"
		"namespacelabs.dev/foundation/universe/db/postgres/server/img":                                "sha256:c300e9112e9a38453959db463b5505a4cd86f4cbc0eb2888f68c82f5deed5645"
		"namespacelabs.dev/foundation/universe/development/localstack/s3/internal/configure":          "sha256:aa2f552d371d3f27e8752e011680db0c18b9ea4cd6d4ddaa93dc6a496079bb3f"
		"namespacelabs.dev/foundation/universe/development/localstack/s3/internal/managebuckets/init": "sha256:4740b806a5fcd81a7d44190265b7dd99a8e028e36b5750d21bd3df9cca63060e"
		"namespacelabs.dev/foundation/universe/networking/tailscale/image":                            "sha256:713a22cbb2582bdb05dcc83101c552677d216321f38ae125dfa11274c3699132"
		"namespacelabs.dev/foundation/universe/storage/s3/internal/managebuckets":                     "sha256:99b6cafd56501e1d96e06889c4fcb2b2a3aab735fbe5d4e0c5acddd8f8e30a5c"
		"namespacelabs.dev/foundation/universe/storage/s3/internal/prepare":                           "sha256:d5c5f500ba0acf6158670d4bd4161d1f601eb4a1c2af27a9930514cf5b40730c"
	}
	baseRepository: "us-docker.pkg.dev/foundation-344819/prebuilts/"
}
