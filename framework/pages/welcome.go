// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package pages

import (
	"embed"
	"encoding/base64"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"namespacelabs.dev/foundation/framework/runtime"
)

var (
	//go:embed logo.png welcome.html.template output.css
	resources embed.FS

	logoBytesBase64, cssStyles string
	welcomeTemplate            *template.Template
)

func init() {
	logoBytesBase64 = base64.StdEncoding.EncodeToString(mustRead("logo.png"))
	cssStyles = string(mustRead("output.css"))

	t, err := template.New("welcome.html").Parse(string(mustRead("welcome.html.template")))
	if err != nil {
		panic(err)
	}
	welcomeTemplate = t
}

type welcomeTemplateData struct {
	PackageName string
	Static      struct {
		Style      template.CSS
		LogoBase64 string
	}
}

func WelcomePage(srv *runtime.Server) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(200)
		pkg := strings.TrimPrefix(srv.PackageName, srv.ModuleName+"/")

		data := welcomeTemplateData{PackageName: pkg}
		data.Static.Style = template.CSS(cssStyles)
		data.Static.LogoBase64 = logoBytesBase64

		if err := welcomeTemplate.Execute(rw, data); err != nil {
			log.Fatal(err)
		}
	}
}

func mustRead(name string) []byte {
	contents, err := fs.ReadFile(resources, name)
	if err != nil {
		panic(err)
	}
	return contents
}
