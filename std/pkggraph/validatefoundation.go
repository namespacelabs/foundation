package pkggraph

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/schema"
)

type LoadModuleFunc func(schema.PackageName) (*Module, error)

func ValidateFoundation(what string, minimumVersion int, lmf LoadModuleFunc) error {
	fn, err := lmf("namespacelabs.dev/foundation")
	if err != nil {
		return fnerrors.InternalError("failed to determine minimum foundation version: %w", err)
	}

	data, err := versions.LoadAtOrDefaults(fn.ReadOnlyFS(), "internal/versions/versions.json")
	if err != nil {
		return fnerrors.InternalError("failed to load foundation version data: %w", err)
	}

	if data.APIVersion < minimumVersion {
		return fnerrors.NamespaceTooRecent(what, int32(minimumVersion), int32(data.APIVersion))
	}

	return nil
}

func ModuleFromLoader(ctx context.Context, pl PackageLoader) LoadModuleFunc {
	return func(pn schema.PackageName) (*Module, error) {
		fn, err := pl.Resolve(ctx, pn)
		if err != nil {
			return nil, fnerrors.InternalError("failed to load module %q: %w", pn, err)
		}
		return fn.Module, nil
	}
}

func ModuleFromModules(mods Modules) LoadModuleFunc {
	return func(pn schema.PackageName) (*Module, error) {
		for _, mod := range mods.Modules() {
			if mod.ModuleName() == pn.String() {
				return mod, nil
			}
		}

		return nil, fnerrors.InternalError("failed to load module: %q: not loaded", pn)
	}
}
