// This file was automatically generated.
// Contains type and function definitions that needs to be implemented in "impl.ts".

import * as impl from "./impl";
import { Registrar } from "@namespacelabs/foundation";

export type ProvideBackend = <T>(outputTypeCtr: new (...args: any[]) => T) =>
		T;
export const provideBackend: ProvideBackend = impl.provideBackend;
