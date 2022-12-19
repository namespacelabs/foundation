providers: {
	"namespacelabs.dev/foundation/internal/testdata/integrations/resources/classes:Database2": {
		prepareWith: {
			imageFrom: binary: "namespacelabs.dev/foundation/internal/testdata/integrations/resources/buildkitprovider"
			env: {
				TEST_SECRET: fromSecret: ":testsecret"
			}
		}
	}
}

secrets: {
	testsecret: {
		description: "A test secret."
	}
}
