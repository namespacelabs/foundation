// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nscloud

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func fetchGithubSshKeys(ctx context.Context, username string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://github.com/%s.keys", username), nil)
	if err != nil {
		return nil, err
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != 200 {
		return nil, fnerrors.InvocationError("unexpected status code %d", response.StatusCode)
	}

	keysData, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(string(keysData), "\n")

	var keys []string
	for _, p := range parts {
		x := strings.TrimSpace(p)
		if x != "" {
			keys = append(keys, x)
		}
	}

	return keys, nil
}

type sshKeys struct {
	username string

	compute.DoScoped[[]string]
}

var _ compute.Computable[[]string] = &sshKeys{}

func (s *sshKeys) Action() *tasks.ActionEvent { return tasks.Action("github.fetch-public-ssh-keys") }
func (s *sshKeys) Inputs() *compute.In        { return compute.Inputs().Str("username", s.username) }
func (s *sshKeys) Output() compute.Output     { return compute.Output{NotCacheable: true} }
func (s *sshKeys) Compute(ctx context.Context, _ compute.Resolved) ([]string, error) {
	return fetchGithubSshKeys(ctx, s.username)
}

func UserSSHKeys() (compute.Computable[[]string], error) {
	user, err := fnapi.LoadUser()
	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(user.Username, "[bot]") {
		return nil, nil
	}

	return &sshKeys{username: user.Username}, nil
}
