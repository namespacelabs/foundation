import "namespacelabs.dev/foundation/std/fn"

test: fn.#Test & {
	name: "test-trascoding"

	binary: {
		from: go_package: "."
	}

	fixture: {
		serversUnderTest: [
			"namespacelabs.dev/foundation/std/testdata/server/gogrpc",
			"namespacelabs.dev/foundation/languages/nodejs/testdata/server",
		]
	}
}
