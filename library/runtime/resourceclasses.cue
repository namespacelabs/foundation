resourceClasses: {
	Server: {
		intent: {
			type: "foundation.schema.PackageRef"
		}
		produces: {
			type:   "foundation.schema.runtime.Server"
			source: "../../schema/runtime/config.proto"
		}
	}
	Secret: {
		intent: {
			type: "foundation.schema.PackageRef"
		}
		produces: {
			type:   "foundation.library.runtime.SecretInstance"
			source: "./secrets.proto"
		}
	}
	Ingress: {
		intent: {
			type:   "foundation.library.runtime.IngressIntent"
			source: "./ingress.proto"
		}
		produces: {
			type:   "foundation.library.runtime.IngressInstance"
			source: "./ingress.proto"
		}
	}
}
