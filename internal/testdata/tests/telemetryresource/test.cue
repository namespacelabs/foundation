import "namespacelabs.dev/foundation/std/fn"

test: fn.#Test & {
	name: "test-telemetry-resource"

	binary: {
		from: go_package: "."
	}

	fixture: {
		sut: "namespacelabs.dev/foundation/internal/testdata/server/telemetryresource"
	}
}
