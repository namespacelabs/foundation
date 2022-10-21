resources: {
	withInput: {
		kind: "namespacelabs.dev/foundation/internal/testdata/integrations/resources/classes:Database"
		on:   "namespacelabs.dev/foundation/internal/testdata/integrations/resources/providers"

		input: {
			name: "Bob"
		}
	}

	emptyInput: {
		kind: "namespacelabs.dev/foundation/internal/testdata/integrations/resources/classes:Database"
		on:   "namespacelabs.dev/foundation/internal/testdata/integrations/resources/providers"
	}

	withInputFrom: {
		kind: "namespacelabs.dev/foundation/internal/testdata/integrations/resources/classes:Database"
		on:   "namespacelabs.dev/foundation/internal/testdata/integrations/resources/providers"

		from: {
			binary: "namespacelabs.dev/foundation/universe/db/postgres/rds/prepare"
		}
	}

	test1: {
		kind: "namespacelabs.dev/foundation/internal/testdata/integrations/resources/classes:Database"
		on:   "namespacelabs.dev/foundation/internal/testdata/integrations/resources/testgenprovider"

		input: {
			name: "helloworld"
		}
	}
}
