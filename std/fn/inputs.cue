package inputs

import fntypes "namespacelabs.dev/foundation/std/fn:types"

#Port: {
	@fn(alloc=port)
	name: string
	// Set by the runtime.
	port?: int
}

#Package: {
	@fn(input=package)
	string
}

#Service: {
	@fn(input=service)

	packageName: #Package

	// Set by the runtime.
	protoTypename?: string
	goPackage?:     string
}

#Server: {
	@fn(input=server_dep)

	packageName: #Package

	// Set by the runtime.
	id?:   string
	name?: string
	endpoints: [...#Endpoint]

	$endpointMap: {
		for endpoint in endpoints {
			"\(endpoint.serviceName)": endpoint
		}
	}

	$addressMap: {
		for endpoint in endpoints {
			"\(endpoint.serviceName)": "\(endpoint.allocatedName):\(endpoint.containerPort)"
		}
	}

	#Endpoint: {
		type:          "PRIVATE" | "INTERNET_FACING"
		serviceName:   string
		allocatedName: string
		containerPort: int
	}
}

#Proto: {
	@fn(input=protoload)

	*{sources: [string, ...string]} | {source: string, sources: [source]}

	// Set by the runtime.
	types: [string]:    fntypes.#Proto
	services: [string]: fntypes.#Proto
}

#FocusServer: {
	@fn(input=focus_server)
	image?:    string
	framework: string
}

#Workspace: {
	@fn(input=workspace)
	moduleName: string
	serverPath: string
}

#Environment: {
	@fn(input=env)
	name:      string
	runtime:   string
	purpose:   "DEVELOPMENT" | "TESTING" | "PRODUCTION"
	ephemeral: bool
}

#FromFile: {
	@fn(input=resource)
	fntypes.#Resource
}
