package fn

import (
	"namespacelabs.dev/foundation/std/fn:types"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

_#Imports: {
	"import": [...string]
}

_#Instantiate: {
	instantiate: [#InstanceName]: {
		...
	}
}

_#Node: {
	_#Imports

	_#Instantiate

	on?: {
		prepare?: {
			{invokeInternal: string} | {invokeBinary: #InvokeBinary}
			requires: [...inputs.#Package]
		}
	}

	packageData: [...string]

	requirePersistentStorage?: {
		persistentId: string
		byteCount:    string
		mountPath:    string
	}

	environment?: {
		required?: [string]: string
	}
}

#Extension: {
	_#Node

	hasInitializerIn?: #Framework | [...#Framework]
	initializeBefore: [...inputs.#Package]
	initializeAfter: [...inputs.#Package]

	provides?: #Provides

	#Provides: [X=string]: {
		_#Instantiate

		name: X
		{input: types.#Proto} | {type: types.#Proto}
		availableIn: [string]: {...}
	}
}

#InstanceName: string

#Service: {
	_#Node

	framework: #Framework

	ingress: *"PRIVATE" | "INTERNET_FACING"

	exportService?:        types.#Proto
	exportServicesAsHttp?: bool // XXX move this to the service definition.
	exportMethods?: {
		service: types.#Proto
		methods: [...string]
	}

	exportHttp?: [...#HttpPath]
}

#HttpPath: {
	path:  string
	kind?: string
}

#Framework: "GO" | "GO_GRPC" | "WEB"

#Server: {
	_#Imports

	id:   string
	name: string

	framework: #Framework | "OPAQUE"

	isStateful?: bool
	testonly?:   bool

	if framework == "OPAQUE" {
		service: [string]: #ServiceSpec
		ingress: [string]: #ServiceSpec
	}

	// XXX temporary
	env: [string]: string

	urlmap: [...#UrlMapEntry]

	#ServiceSpec: {
		name?:         string
		label?:        string
		containerPort: int
		metadata: {
			kind?:    string
			protocol: string
		}
		internal: *false | true

		experimentalAdditionalMetadata?: [...{
			kind?:     string
			protocol?: string
			experimentalDetails?: {
				typeUrl: string
				body:    string
			}
		}]
	}

	#UrlMapEntry: {
		path:    string
		import?: inputs.#Package
	}

	#Naming: {
		withOrg?: string
	}
}

#OpaqueServer: {
	#Server

	framework: "OPAQUE"

	binary: *{
		image: string
	} | inputs.#Package
}

#Image: {
	prebuilt?: string
	src?:      #BuildPlan // XXX validation is done by the Go runtime at the moment.
}

#BuildPlan: {
	buildFile?: string
	imageRoot:  *"." | string
	hermetic:   *false | true
	...
}

#OpaqueBinary: {
	#Image
	command: [...string]
	... // XXX not a real fan of leaving this open; but need to if want extensions to the binary definition.
}

#Args: [string]: string

#WithPackageName: {
	packageName: inputs.#Package
	...
}

_#ConfigureBase: {
	stack?: {
		append: [...#WithPackageName]
	}
	startup?: #Startup
	sidecar?: {[string]: #Container}
	init?: {[string]: #Container}
	naming?: #Naming
	...

	provisioning?: #Provisioning
	#Provisioning: {
		// XXX add purpose, e.g. contributes startup inputs.
		with?: {
			#InvokeBinary
			snapshot: [string]: {fromWorkspace: string}
			noCache:      *false | true
			requiresKeys: *false | true
		}
	}

	#Startup: {
		args?: #Args | [...string]
		env: [string]: string
	}

	#Container: {
		binary: inputs.#Package
		args:   #Args
	}

	#Naming: {
		withOrg?: string
		*{} | {domainName: [string]: [...string]} | {tlsManagedDomainName: [string]: [...string]}
	}
}

#InvokeBinary: {
	imageFrom:  #ImageFrom
	args:       #Args
	workingDir: *"/" | string
}

#ImageFrom: {
	binary: inputs.#Package
}

#Configure: _#ConfigureBase & {
	with?: #Invocation
}

// Deprecated.
#Extend: _#ConfigureBase & {
	provisioning?: {
		with?: #Invocation
	}
}

// XXX add purpose, e.g. contributes startup inputs.
#Invocation: {
	binary:     inputs.#Package
	args:       #Args
	workingDir: *"/" | string
	snapshot: [string]: {
		fromWorkspace: string
		optional:      *false | true
		requireFile:   *false | true
	}
	noCache:      *false | true
	requiresKeys: *false | true
	inject: [...string]
}

#Binary: {
	name?:       string
	repository?: string
	digest?:     string
	{from: #BuildPlan} | {
		build_plan: {
			layer_build_plan: [...#BuildPlan]
		} | [...#BuildPlan]
	}

	#BuildPlan: {
		go_package?: string
		go_build?: {
			rel_path:     string
			binary_name:  string
			binary_only?: bool
		}
		dockerfile?: string
		nix_flake?:  string
		web_build?:  string
		llb_plan?: {output_of: #Binary}
		alpine_build?: {package?: [...string]}
		files?: [...string]
		snapshot_files?: [...string]
	}

	config?: {
		command?: [...string]
		// XXX enable when they can also be used by all binaries.
		// args?: #Args
		// env?: map[string]string
	}
}

#Test: {
	name: string
	*{driver: #Binary} | {binary: #Binary}
	fixture: {
		sut: inputs.#Package
		serversUnderTest: [sut]
	} | *{
		serversUnderTest: [inputs.#Package, ...inputs.#Package]
	}
}
