// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package github

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"text/template"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v45/github"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
)

func newUpdateStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "update-status",
	}

	flag := cmd.Flags()

	// Required flags:
	installationID := flag.Int64("installation_id", -1, "Installation ID that we're requesting an access token to.")
	appID := flag.Int64("app_id", -1, "app ID of the app we're requesting an access token to.")
	privateKey := flag.String("private_key", "", "Path to the app's private key.")
	owner := flag.String("owner", "", "Organization name.")
	repo := flag.String("repo", "", "Repository name.")
	commit := flag.String("commit", "", "Commit's SHA.")

	// Optional - setting commit's status.
	status := flag.String("status", "", "Sets the status of the commit to either pending/success/error/failure")
	statusDescription := flag.String("status_description", "", "Sets the description of the status")

	// Optional - adding a comment to a commit.
	deployOutput := flag.String("deploy_output", "", "Structured data for de")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, *appID, *installationID, *privateKey)
		if err != nil {
			return err
		}

		client := github.NewClient(&http.Client{Transport: itr})
		if *status != "" {
			if _, _, err := client.Repositories.CreateStatus(ctx, *owner, *repo, *commit, &github.RepoStatus{
				State:       github.String(*status),
				Description: github.String(*statusDescription),
				Context:     github.String("namespace-ci/autopush"),
			}); err != nil {
				return err
			}
		}

		if *deployOutput != "" {
			comment, err := decodeJSON(*deployOutput)
			if err != nil {
				return err
			}
			if _, _, err := client.Repositories.CreateComment(ctx, *owner, *repo, *commit, &github.RepositoryComment{
				Body: github.String(comment),
			}); err != nil {
				return err
			}
		}

		return nil
	})

	return cmd
}

var (
	mainTmpl = template.Must(template.New("template").Parse(`
**Deployments**
{{range $k, $ingress := .Ingress}}
- [x] ({{range $k, $protocol := $ingress.Protocol}}**{{$protocol}}**{{end}}) {{ $ingress.Owner }}: {{ $ingress.Fdqn }}
{{end}}
`))
)

func decodeJSON(jsonFile string) (string, error) {
	reader, err := os.Open(jsonFile)
	if err != nil {
		return "", err
	}
	output := cmd.Output{}
	if err := json.NewDecoder(reader).Decode(&output); err != nil {
		return "", err
	}

	var body bytes.Buffer
	if err := mainTmpl.Execute(&body, output); err != nil {
		return "", err
	}

	return body.String(), nil
}
