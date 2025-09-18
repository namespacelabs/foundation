import "namespacelabs.dev/foundation/std/fn"

test: fn.#Test & {
	name: "test-cockroach"

	binary: {
		from: go_package: "."
	}

	fixture: {
		sut: "namespacelabs.dev/foundation/internal/testdata/server/cockroach"
	}
}
