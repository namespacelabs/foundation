import "namespacelabs.dev/foundation/std/fn"

test: fn.#Test & {
	name: "test-can-scrape"

	binary: {
		from: go_package: "."
	}

	fixture: {
		sut: "namespacelabs.dev/foundation/internal/testdata/server/gogrpc"
	}
}
