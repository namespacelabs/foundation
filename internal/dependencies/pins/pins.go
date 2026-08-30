// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package pins

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

var (
	//go:embed pins.json
	lib embed.FS
)

type versionsJSON struct {
	Images      map[string]string     `json:"images"`
	Defaults    map[string]string     `json:"defaults"`
	ServerBases map[string]ServerBase `json:"serverBases"`
}

type ServerBase struct {
	Base          string `json:"base"`
	NonRootUserID *int   `json:"nonRootUserId"`
	FSGroup       *int   `json:"fsGroup"`
}

var (
	versions versionsJSON
)

func Default(name string) string {
	r, err := CheckDefault(name)
	if err != nil {
		panic(err.Error())
	}
	return r
}

func Image(image string) string {
	r, err := CheckImage(image)
	if err != nil {
		panic(err.Error())
	}
	return r
}

func Server(name string) *ServerBase {
	srv, ok := versions.ServerBases[name]
	if ok {
		return &srv
	}
	return nil
}

func CheckDefault(name string) (string, error) {
	image := versions.Defaults[name]
	if image == "" {
		return "", fnerrors.InternalError("%q is not present in defaults", name)
	}
	digest := versions.Images[image]
	if digest == "" {
		return "", fnerrors.InternalError("%q of default %q is not pinned", image, name)
	}
	return fmt.Sprintf("%s@sha256:%s", image, digest), nil
}

func CheckImage(image string) (string, error) {
	digest := versions.Images[image]
	if digest == "" {
		return "", fnerrors.InternalError("%q is not pinned", image)
	}
	return fmt.Sprintf("%s@sha256:%s", image, digest), nil
}

func init() {
	versionData, err := fs.ReadFile(lib, "pins.json")
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(versionData, &versions); err != nil {
		panic(err)
	}
}
