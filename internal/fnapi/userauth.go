// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnapi

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"namespacelabs.dev/foundation/workspace/dirs"
)

var ErrRelogin = errors.New("not logged in, please run `ns login`")

type UserAuth struct {
	Username string `json:"username"`
	Org      string `json:"org"` // The organization this user is acting as. Only really relevant for robot accounts which authenticate against a repository.
	Opaque   []byte `json:"opaque"`
}

const userAuthJson = "auth.json"

func LoginAsRobotAndStore(ctx context.Context, repository, accessToken string) (string, error) {
	userAuth, err := RobotLogin(ctx, repository, accessToken)
	if err != nil {
		return "", err
	}

	return StoreUser(ctx, userAuth)
}

func StoreUser(ctx context.Context, userAuth *UserAuth) (string, error) {
	userAuthData, err := json.Marshal(userAuth)
	if err != nil {
		return "", err
	}

	configDir, err := dirs.Ensure(dirs.Config())
	if err != nil {
		return "", err
	}

	if err := ioutil.WriteFile(filepath.Join(configDir, userAuthJson), userAuthData, 0600); err != nil {
		return "", err
	}

	return userAuth.Username, nil
}

func LoadUser() (*UserAuth, error) {
	dir, err := dirs.Config()
	if err != nil {
		return nil, err
	}

	p := filepath.Join(dir, userAuthJson)
	data, err := ioutil.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			// XXX use fnerrors
			return nil, ErrRelogin
		}

		return nil, err
	}

	userAuth := &UserAuth{}
	if err := json.Unmarshal(data, userAuth); err != nil {
		return nil, err
	}

	return userAuth, nil
}
