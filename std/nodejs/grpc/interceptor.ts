import { ServiceClientConstructor, Server } from "@grpc/grpc-js";

// register<RequestType, ResponseType>(
//   name: string,
//   handler: HandleCall<RequestType, ResponseType>,
//   serialize: Serialize<ResponseType>,
//   deserialize: Deserialize<RequestType>,
//   type: string): boolean
export type ServerHandlerRegisterFunction = typeof Server.prototype.register;

export interface GrpcInterceptorRegistrar {
	interceptClientConstructor(
		interceptor: (constructor: ServiceClientConstructor) => ServiceClientConstructor
	): void;

	interceptServerHandlerRegistration(
		interceptor: (originalRegister: ServerHandlerRegisterFunction) => ServerHandlerRegisterFunction
	): void;
}
