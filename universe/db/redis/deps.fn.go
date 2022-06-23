// This file was automatically generated by Foundation.
// DO NOT EDIT. To update, re-run `ns generate`.

package redis

import (
	"context"
	"github.com/go-redis/redis/v8"
	fncore "namespacelabs.dev/foundation/std/core"
	"namespacelabs.dev/foundation/std/go/core"
)

// Dependencies that are instantiated once for the lifetime of the extension.
type ExtensionDeps struct {
	ReadinessCheck core.Check
}

type _checkProvideRedis func(context.Context, *RedisArgs, ExtensionDeps) (*redis.Client, error)

var _ _checkProvideRedis = ProvideRedis

var (
	Package__1i719d = &core.Package{
		PackageName: "namespacelabs.dev/foundation/universe/db/redis",
	}

	Provider__1i719d = core.Provider{
		Package:     Package__1i719d,
		Instantiate: makeDeps__1i719d,
	}
)

func makeDeps__1i719d(ctx context.Context, di core.Dependencies) (_ interface{}, err error) {
	var deps ExtensionDeps

	if deps.ReadinessCheck, err = fncore.ProvideReadinessCheck(ctx, nil); err != nil {
		return nil, err
	}

	return deps, nil
}
