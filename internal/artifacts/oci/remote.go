// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/dustin/go-humanize"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"golang.org/x/exp/maps"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/versions"
)

var (
	staticMapping = []keychainMap{}

	mirrorPortMap = map[string]string{
		"http":  "80",
		"https": "443",
	}

	UsePercentageInTracking = false
)

func DockerHubMirror() string {
	return os.Getenv("NS_DOCKERHUB_MIRROR")
}

type KeychainWhen int

const (
	Keychain_UseAlways KeychainWhen = iota
	Keychain_UseOnWrites
)

type keychainMap struct {
	Domain   string
	Keychain Keychain
	When     KeychainWhen
}

func RegisterDomainKeychain(suffix string, keychain Keychain, purpose KeychainWhen) {
	staticMapping = append(staticMapping, keychainMap{suffix, keychain, purpose})
}

func userAgent() remote.Option {
	return remote.WithUserAgent(fmt.Sprintf("NamespaceCLI/%d", versions.Builtin().APIVersion))
}

func RemoteOptsWithAuth(ctx context.Context, access RegistryAccess, writeAccess bool) ([]remote.Option, error) {
	options := []remote.Option{
		remote.WithContext(ctx),
		userAgent(),
	}

	if !access.PublicImage {
		options = append(options, remote.WithAuthFromKeychain(keychainSequence{ctx, access.Keychain, writeAccess}))
	}

	transportOptions, err := parseTransport(ctx, access.Transport)
	if err != nil {
		return nil, err
	}

	options = append(options, transportOptions...)

	return options, nil
}

func ParseRefAndKeychain(ctx context.Context, imageRef string, opts RegistryAccess) (name.Reference, []remote.Option, error) {
	var nameOpts []name.Option
	if opts.InsecureRegistry {
		nameOpts = append(nameOpts, name.Insecure)
	}

	ref, err := name.ParseReference(imageRef, nameOpts...)
	if err != nil {
		return nil, nil, err
	}

	options, err := RemoteOptsWithAuth(ctx, opts, false)
	if err != nil {
		return nil, nil, err
	}

	ref, err = RefWithRegistryMirror(ctx, ref)
	if err != nil {
		return nil, nil, err
	}

	return ref, options, nil
}

func RefWithRegistryMirror(ctx context.Context, imageRef name.Reference) (name.Reference, error) {
	// Check if image registry is `index.docker.io` and docker hub mirror is provided.
	if imageRef.Context().RegistryStr() != name.DefaultRegistry || DockerHubMirror() == "" {
		return imageRef, nil
	}

	mirrorURL, err := url.Parse(DockerHubMirror())
	if err != nil {
		return nil, err
	}

	defaultMirrorPort, ok := mirrorPortMap[mirrorURL.Scheme]
	if !ok {
		return nil, fnerrors.Newf("docker hub mirror scheme %q is not supported; supported values: %s",
			mirrorURL.Scheme, strings.Join(maps.Keys(mirrorPortMap), ","))
	}

	var mirrorOpts []name.Option
	if mirrorURL.Scheme == "http" {
		mirrorOpts = append(mirrorOpts, name.Insecure)
	}

	mirrorHost := mirrorURL.Host
	if mirrorURL.Port() == "" {
		mirrorHost = net.JoinHostPort(mirrorHost, defaultMirrorPort)
	}

	fmt.Fprintf(console.Debug(ctx), "using mirror %q for registry %q\n", DockerHubMirror(), name.DefaultRegistry)

	imageRepo := imageRef.Context()
	// We create a new repository here to have full name of repository (e.g. including 'library').
	mirrorRepo, err := name.NewRepository(fmt.Sprintf("%s/%s", mirrorHost, imageRepo.RepositoryStr()), mirrorOpts...)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(console.Debug(ctx), "using repository %q\n", mirrorRepo.Name())
	return &imageReference{
		Reference:  imageRef,
		repository: mirrorRepo,
	}, nil
}

type keychainSequence struct {
	ctx      context.Context
	provided Keychain
	writing  bool
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

	return defaultKeychain{ks.ctx, ks.writing}.Resolve(resource)
}

type defaultKeychain struct {
	ctx     context.Context
	writing bool
}

func (dk defaultKeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	useKeychain := func(k keychainMap) bool {
		return k.When == Keychain_UseAlways || (k.When == Keychain_UseOnWrites && dk.writing)
	}

	// First we search for an exact match of a registry.
	for _, kc := range staticMapping {
		if resource.RegistryStr() == kc.Domain && useKeychain(kc) {
			return kc.Keychain.Resolve(dk.ctx, resource)
		}
	}

	// Then for parent domain match, useful for registering Cloud container registries (e.g. per account registries in
	// amazonaws.com or pkg.dev) to use the same keychain for all the AWS or GCP registries.
	for _, kc := range staticMapping {
		if strings.HasSuffix(resource.RegistryStr(), "."+kc.Domain) && useKeychain(kc) {
			return kc.Keychain.Resolve(dk.ctx, resource)
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

	if UsePercentageInTracking {
		var percent float64
		if total == 0 {
			percent = 0
		} else {
			percent = float64(complete) / float64(total) * 100
		}

		return fmt.Sprintf("%.2f%%", percent)
	}

	if total == 0 {
		return humanize.Bytes(uint64(complete))
	}

	return fmt.Sprintf("%s / %s", humanize.Bytes(uint64(complete)), humanize.Bytes(uint64(total)))
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
