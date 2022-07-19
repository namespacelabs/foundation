import "namespacelabs.dev/foundation/std/fn"

test: fn.#Test & {
	name: "test-postgres"

	binary: {
		from: go_package: "."
	}

	fixture: {
		sut: "namespacelabs.dev/foundation/std/testdata/server/postgres"
	}
}
