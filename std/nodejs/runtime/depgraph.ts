export interface DepsFactory<T> {
	key: string;
	instantiate: (dg: DependencyGraph) => T;
}

export class DependencyGraph {
	instantiate<P, ScopedDepsT, SingletonDepsT>(params: {
		scopedDepsFactory?: DepsFactory<ScopedDepsT>;
		singletonDepsFactory?: DepsFactory<SingletonDepsT>;
		providerFn: (params: { scopedDeps?: ScopedDepsT; singletonDeps?: SingletonDepsT }) => P;
	}): P {
		return params.providerFn({
			scopedDeps: params.scopedDepsFactory?.instantiate(this),
			singletonDeps: params.singletonDepsFactory?.instantiate(this),
		});
	}
}
