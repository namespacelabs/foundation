// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

var (
	staticMapping = []keychainMap{}
)

type keychainMap struct {
	Suffix   string
	Keychain Keychain
}

func RegisterDomainKeychain(suffix string, keychain Keychain) {
	staticMapping = append(staticMapping, keychainMap{"." + suffix, keychain})
}

func RemoteOpts(ctx context.Context) []remote.Option {
	return []remote.Option{remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain)}
}

func RemoteOptsWithAuth(ctx context.Context, keychain Keychain) []remote.Option {
	if keychain == nil {
		return RemoteOpts(ctx)
	}
	return []remote.Option{remote.WithContext(ctx), remote.WithAuthFromKeychain(keychainSequence{ctx, keychain})}
}

type keychainSequence struct {
	ctx      context.Context
	provided Keychain
}

func (ks keychainSequence) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	if ks.provided != nil {
		res, err := ks.provided.Resolve(ks.ctx, resource)
		if err != nil {
			return nil, err
		}

		if res != nil {
			return res, nil
		}
	}

	return defaultKeychain{ks.ctx}.Resolve(resource)
}

type defaultKeychain struct {
	ctx context.Context
}

func (ks defaultKeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	for _, kc := range staticMapping {
		if strings.HasSuffix(resource.RegistryStr(), kc.Suffix) {
			return kc.Keychain.Resolve(ks.ctx, resource)
		}
	}

	return authn.DefaultKeychain.Resolve(resource)
}

type RemoteProgress struct {
	mu              sync.RWMutex
	tracking        bool
	complete, total int64
}

func (rp *RemoteProgress) FormatProgress() string {
	rp.mu.RLock()
	tracking := rp.tracking
	complete, total := rp.complete, rp.total
	rp.mu.RUnlock()
	if !tracking {
		return ""
	}

	var percent float64
	if total == 0 {
		percent = 0
	} else {
		percent = float64(complete) / float64(total) * 100
	}

	return fmt.Sprintf("%.2f%%", percent)
}

func (rp *RemoteProgress) Track() remote.Option {
	rp.mu.Lock()
	rp.tracking = true
	rp.mu.Unlock()

	ch := make(chan v1.Update, 1)

	go func() {
		for update := range ch {
			rp.mu.Lock()
			rp.complete = update.Complete
			rp.total = update.Total
			rp.mu.Unlock()
		}
	}()

	return remote.WithProgress(ch)
}
