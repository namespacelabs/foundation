providers: {
	"namespacelabs.dev/foundation/integrations/testdata/resources/classes:Database": {
		initializedWith: {
			binary: "namespacelabs.dev/foundation/integrations/testdata/resources/testgenprovider/testgen"
		}

		resources: {
			another: {
				kind: "namespacelabs.dev/foundation/std/runtime:Server"
				input: package_name: "namespacelabs.dev/foundation/integrations/testdata/dockerfile/simple"
			}

			secretres: {
				kind: "namespacelabs.dev/foundation/std/runtime:Secret"
				input: ref: ":key1"
			}
		}
	}
}

secrets: {
	key1: {
		description: "A generated secret, for testing purposes."
		generate: {
			uniqueId:        "testgen-key"
			randomByteCount: 16
		}
	}
}
