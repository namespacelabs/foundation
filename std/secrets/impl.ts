import { Secret, Value } from "@namespacelabs.dev-foundation/std-secrets/provider_pb";

export async function provideSecret(req: Secret): Promise<Value> {
	return { name: "secret", path: "secret" };
}
export interface Value2 {}
