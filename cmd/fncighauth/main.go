// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/bradleyfalzon/ghinstallation/v2"
)

var (
	installationID = flag.Int64("installation_id", -1, "Installation ID that we're requesting an access token to.")
	appID          = flag.Int64("app_id", -1, "app ID of the app we're requesting an access token to.")
	privateKey     = flag.String("private_key", "", "Path to the app's private key.")
)

func main() {
	flag.Parse()

	if err := Do(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func Do(ctx context.Context) error {
	itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, *appID, *installationID, *privateKey)
	if err != nil {
		return err
	}

	token, err := itr.Token(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, token)
	return nil
}
