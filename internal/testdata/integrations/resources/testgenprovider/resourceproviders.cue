providers: {
	"namespacelabs.dev/foundation/internal/testdata/integrations/resources/classes:Database": {
		initializedWith: imageFrom: binary: "namespacelabs.dev/foundation/internal/testdata/integrations/resources/testgenprovider/testgen"

		intent: {
			type:   "foundation.internal.testdata.integrations.resources.classes.DatabaseIntent"
			source: "./proto1.proto"
		}

		resources: {
			another: {
				kind:  "namespacelabs.dev/foundation/library/runtime:Server"
				input: "namespacelabs.dev/foundation/internal/testdata/integrations/dockerfile/simple"
			}

			secrets: {
				kind:  "namespacelabs.dev/foundation/library/runtime:Secret"
				input: ":key1"
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
