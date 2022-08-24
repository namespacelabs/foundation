// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package supertokens

import (
	"context"
	"os"

	"github.com/supertokens/supertokens-golang/recipe/passwordless"
	"github.com/supertokens/supertokens-golang/recipe/passwordless/plessmodels"
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/thirdparty"
	"github.com/supertokens/supertokens-golang/recipe/thirdparty/tpmodels"
	"github.com/supertokens/supertokens-golang/recipe/thirdpartyemailpassword"
	"github.com/supertokens/supertokens-golang/recipe/thirdpartyemailpassword/tpepmodels"
	"github.com/supertokens/supertokens-golang/recipe/thirdpartypasswordless"
	"github.com/supertokens/supertokens-golang/recipe/thirdpartypasswordless/tplmodels"
	"github.com/supertokens/supertokens-golang/supertokens"
)

func Prepare(ctx context.Context, deps ExtensionDeps) error {
	os.Setenv("SUPERTOKENS_DEBUG", "true")

	deps.Middleware.Add(supertokens.Middleware)

	apiBasePath := "/auth"
	websiteBasePath := "/auth"
	return supertokens.Init(supertokens.TypeInput{
		Supertokens: &supertokens.ConnectionInfo{
			// These are the connection details of the app you created on supertokens.com
			ConnectionURI: "https://e4f2b9e11e2111ed9d278f2c02603027-us-east-1.aws.supertokens.io:3570",
			APIKey:        "",
		},
		AppInfo: supertokens.AppInfo{
			AppName: "Namespace",
			// APIDomain:       "https://login.namespacelabs.so",
			// WebsiteDomain:   "https://dashboard.namespace.so",
			APIDomain:       "http://signin.dev.nslocal.host:40080",
			WebsiteDomain:   "http://signin.dev.nslocal.host:40080",
			APIBasePath:     &apiBasePath,
			WebsiteBasePath: &websiteBasePath,
		},
		RecipeList: []supertokens.Recipe{
			thirdpartyemailpassword.Init(&tpepmodels.TypeInput{
				Providers: []tpmodels.TypeProvider{
					thirdparty.Github(tpmodels.GithubConfig{
						IsDefault:    true,
						ClientID:     string(deps.GithubClientId.MustValue()),
						ClientSecret: string(deps.GithubClientSecret.MustValue()),
					}),
				},
			}),
			passwordless.Init(plessmodels.TypeInput{
				FlowType: "MAGIC_LINK",
				ContactMethodEmail: plessmodels.ContactMethodEmailConfig{
					Enabled: true,
				},
			}),
			thirdpartypasswordless.Init(tplmodels.TypeInput{
				FlowType: "MAGIC_LINK",
				ContactMethodEmail: plessmodels.ContactMethodEmailConfig{
					Enabled: true,
				},
				Providers: []tpmodels.TypeProvider{
					thirdparty.Github(tpmodels.GithubConfig{
						IsDefault:    true,
						ClientID:     string(deps.GithubClientId.MustValue()),
						ClientSecret: string(deps.GithubClientSecret.MustValue()),
					}),
				},
			}),
			session.Init(nil), // initializes session features
		},
	})
}
