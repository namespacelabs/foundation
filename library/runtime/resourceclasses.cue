resourceClasses: {
	Server: {
		produces: {
			type:   "foundation.schema.runtime.Server"
			source: "../../schema/runtime/config.proto"
		}
		defaultProvider: "namespacelabs.dev/foundation/library/runtime"
	}
	Secret: {
		produces: {
			type:   "foundation.library.runtime.SecretInstance"
			source: "./secrets.proto"
		}
		defaultProvider: "namespacelabs.dev/foundation/library/runtime"
	}
	Ingress: {
		produces: {
			type:   "foundation.library.runtime.IngressInstance"
			source: "./ingress.proto"
		}
	}
}

providers: {
	"namespacelabs.dev/foundation/library/runtime:Server": {
		intent: {
			type: "foundation.schema.PackageRef"
		}

		// Implementation is embedded in Namespace itself.
	}
	"namespacelabs.dev/foundation/library/runtime:Secret": {
		intent: {
			type: "foundation.schema.PackageRef"
		}

		// Implementation is embedded in Namespace itself.
	}
}
