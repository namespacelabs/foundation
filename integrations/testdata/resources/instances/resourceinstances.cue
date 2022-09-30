resources: {
	withInput: {
		kind: "namespacelabs.dev/foundation/integrations/testdata/resources/classes:Database"
		on:   "namespacelabs.dev/foundation/integrations/testdata/resources/providers"

		input: {
			name: "Bob"
		}
	}

	emptyInput: {
		kind: "namespacelabs.dev/foundation/integrations/testdata/resources/classes:Database"
		on:   "namespacelabs.dev/foundation/integrations/testdata/resources/providers"
	}

	withInputFrom: {
		kind: "namespacelabs.dev/foundation/integrations/testdata/resources/classes:Database"
		on:   "namespacelabs.dev/foundation/integrations/testdata/resources/providers"

		from: {
			binary: "namespacelabs.dev/foundation/universe/db/postgres/rds/prepare"
		}
	}

	test1: {
		kind: "namespacelabs.dev/foundation/integrations/testdata/resources/classes:Database"
		on:   "namespacelabs.dev/foundation/integrations/testdata/resources/testgenprovider"
	}
}
