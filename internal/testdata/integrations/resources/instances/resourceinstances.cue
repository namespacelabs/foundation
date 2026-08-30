resources: {
	test1: {
		kind:     "namespacelabs.dev/foundation/internal/testdata/integrations/resources/classes:Database"
		provider: "namespacelabs.dev/foundation/internal/testdata/integrations/resources/testgenprovider"

		input: {
			name: "helloworld"
		}
	}

	test2: {
		kind:     "namespacelabs.dev/foundation/internal/testdata/integrations/resources/classes:Database2"
		provider: "namespacelabs.dev/foundation/internal/testdata/integrations/resources/providers"

		input: {
			name: "helloworld"
		}
	}
}
