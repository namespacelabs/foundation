// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/moby/buildkit/client"
)

var (
	ImportCacheVar = &cacheVar{}
	ExportCacheVar = &cacheVar{}
)

type cacheVar struct {
	cacheType string
	args      map[string]string
}

func (c *cacheVar) String() string {
	if c.cacheType == "" {
		return ""
	}
	p := []string{"type=" + c.cacheType}
	for key, val := range c.args {
		p = append(p, fmt.Sprintf("%s=%s", key, val))
	}
	return strings.Join(p, ",")
}

func (c *cacheVar) Set(v string) error {
	args := map[string]string{}
	for _, p := range strings.Split(v, ",") {
		kv := strings.SplitN(p, "=", 2)
		args[kv[0]] = kv[1]
	}
	if args["type"] == "" {
		return errors.New("type is required")
	}
	c.cacheType = args["type"]
	delete(args, "type")
	c.args = args
	return nil
}

func (*cacheVar) Type() string { return "" }

func fillInCaching(sopt *client.SolveOpt) {
	if ImportCacheVar.cacheType != "" {
		sopt.CacheImports = append(sopt.CacheImports, checkUseGithubCache(client.CacheOptionsEntry{
			Type:  ImportCacheVar.cacheType,
			Attrs: ImportCacheVar.args,
		}))
	}
	if ExportCacheVar.cacheType != "" {
		sopt.CacheExports = append(sopt.CacheExports, checkUseGithubCache(client.CacheOptionsEntry{
			Type:  ExportCacheVar.cacheType,
			Attrs: ExportCacheVar.args,
		}))
	}
}

func checkUseGithubCache(entry client.CacheOptionsEntry) client.CacheOptionsEntry {
	if entry.Type == "gha" {
		token, ok1 := os.LookupEnv("ACTIONS_RUNTIME_TOKEN")
		cacheURL, ok2 := os.LookupEnv("ACTIONS_CACHE_URL")

		if !ok1 || !ok2 {
			log.Fatal("buildkit: when cache.type is set to gha, ACTIONS_RUNTIME_TOKEN and ACTIONS_CACHE_URL are required.")
		}

		newEntry := client.CacheOptionsEntry{
			Type: entry.Type,
			Attrs: map[string]string{
				"token": token,
				"url":   cacheURL,
				"scope": entry.Attrs["scope"],
			},
		}

		if newEntry.Attrs["scope"] == "" {
			newEntry.Attrs["scope"] = "fn-buildkit"
		}

		return newEntry
	}

	return entry
}