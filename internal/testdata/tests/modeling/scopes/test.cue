import "namespacelabs.dev/foundation/std/fn"

test: fn.#Test & {
	name: "test-scoped-instantiation"

	binary: {
		from: go_package: "."
	}

	fixture: {
		sut: "namespacelabs.dev/foundation/internal/testdata/server/modeling"
	}
}
