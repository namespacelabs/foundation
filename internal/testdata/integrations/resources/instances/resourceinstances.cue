resources: {
	emptyInput: {
		kind: "namespacelabs.dev/foundation/internal/testdata/integrations/resources/classes:Database"
		on:   "namespacelabs.dev/foundation/internal/testdata/integrations/resources/providers"
	}

	test1: {
		kind: "namespacelabs.dev/foundation/internal/testdata/integrations/resources/classes:Database"
		on:   "namespacelabs.dev/foundation/internal/testdata/integrations/resources/testgenprovider"

		input: {
			name: "helloworld"
		}
	}
}
