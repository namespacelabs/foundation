// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
	"namespacelabs.dev/foundation/framework/resources/provider"
	redisclass "namespacelabs.dev/foundation/library/database/redis"
	redisprovider "namespacelabs.dev/foundation/library/oss/redis"
)

const providerPkg = "namespacelabs.dev/foundation/library/oss/redis"

func main() {
	ctx, p := provider.MustPrepare[*redisprovider.DatabaseIntent]()

	cluster := &redisclass.ClusterInstance{}
	if err := p.Resources.Unmarshal(fmt.Sprintf("%s:cluster", providerPkg), cluster); err != nil {
		log.Fatalf("unable to read required resource \"cluster\": %v", err)
	}

	instance := &redisclass.DatabaseInstance{
		Database:       p.Intent.Database,
		ClusterAddress: cluster.Address,
		Password:       cluster.Password,
		ConnectionUri:  connectionUri(cluster, p.Intent.Database),
	}

	client := redis.NewClient(&redis.Options{
		Network:  "tcp",
		Addr:     instance.ClusterAddress,
		Password: instance.Password,
		DB:       int(instance.Database),
	})

	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis database never became ready: %v", err)
	}

	p.EmitResult(instance)
}

func connectionUri(cluster *redisclass.ClusterInstance, db int32) string {
	return fmt.Sprintf("redis://:%s@%s/%d", cluster.Password, cluster.Address, db)
}
