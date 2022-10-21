resourceClasses: {
	"Database": {
		intent: {
			type:   "foundation.internal.testdata.integrations.resources.classes.DatabaseIntent"
			source: "./proto1.proto"
		}
		produces: {
			type:   "foundation.internal.testdata.integrations.resources.classes.protos.DatabaseInstance"
			source: "./protos/proto2.proto"
		}
	}
}
