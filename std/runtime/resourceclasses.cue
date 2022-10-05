resourceClasses: {
	"Server": {
		intent: {
			type:   "foundation.std.runtime.ServerIntent"
			source: "./intents.proto"
		}
		produces: {
			type:   "foundation.std.runtime.Server"
			source: "./config.proto"
		}
	}
}
