// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package redis

import (
	"context"
	"errors"
	"flag"
	"fmt"

	"github.com/go-redis/redis/v8"
	"namespacelabs.dev/foundation/std/go/core"
)

var redisServerEndpoint = flag.String("redis_endpoint", "", "Redis endpoint address.")

func ProvideRedis(ctx context.Context, args *RedisArgs, deps ExtensionDeps) (*redis.Client, error) {
	if *redisServerEndpoint == "" {
		return nil, errors.New("redis_endpoint is required")
	}

	client := redis.NewClient(&redis.Options{
		Network:  "tcp",
		Addr:     *redisServerEndpoint,
		Password: "",
		DB:       int(args.Database),
	})

	// Asynchronously wait until a database connection is ready.
	deps.ReadinessCheck.RegisterFunc(fmt.Sprintf("redis/%s", core.InstantiationPathFromContext(ctx)), func(ctx context.Context) error {
		return client.Ping(ctx).Err()
	})

	return client, nil
}
