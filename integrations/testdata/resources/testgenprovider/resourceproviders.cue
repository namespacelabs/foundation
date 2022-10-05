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
		}
	}
}
