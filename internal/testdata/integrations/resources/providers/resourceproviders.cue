providers: {
	"namespacelabs.dev/foundation/internal/testdata/integrations/resources/classes:Database2": {
		prepareWith: {
			imageFrom: binary: "namespacelabs.dev/foundation/internal/testdata/integrations/resources/buildkitprovider"
			env: {
				TEST_SECRET: fromSecret: ":testsecret"
			}
		}

		intent: {
			type:   "foundation.internal.testdata.integrations.resources.classes.DatabaseIntent"
			source: "../testgenprovider/proto1.proto"
		}
	}
}

secrets: {
	testsecret: {
		description: "A test secret."
	}
}
