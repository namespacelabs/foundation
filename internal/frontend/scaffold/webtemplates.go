// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package scaffold

import "text/template"

type webServiceTmplOptions struct {
}

type webTemplateFile struct {
	filename string
	tmpl     *template.Template
}

var webTemplates = []webTemplateFile{
	{
		filename: "src/index.tsx",
		tmpl: template.Must(template.New("index.tsx").Parse(`
import React, { useEffect, useState } from "react";
import ReactDOM from "react-dom";
import { Backends } from "../config/backends.fn.js";

ReactDOM.render(
	<React.StrictMode>
			<App />
	</React.StrictMode>,
	document.getElementById("app")
);

function App() {
  const [text, setText] = useState("");

  useEffect(() => {
    const fetchData = async () => {
      const result = await postAPI({ text: "Hello World" });
      const json = await result.json();

      setText(json["text"]);
    };

    fetchData();
  }, []);

  return <>Response from the backend: {text}</>;
}

function postAPI(request: any) {
  return fetch(` + "`" + `${Backends.apiBackend.managed}/api.echoservice.EchoService/Echo` + "`" + `, {
    method: "POST",
    mode: "cors",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(request),
  });
}
`)),
	},
	{
		filename: "package.json",
		tmpl: template.Must(template.New("package.json").Parse(`{
	"private": true,
	"browserslist": "> 0.5%, last 2 versions, not dead",
	"scripts": {
		"dev": "vite",
		"build": "vite build"
	},
	"devDependencies": {
		"@types/react": "^17.0.35",
		"@types/react-dom": "^17.0.11",
		"@vitejs/plugin-react": "^1.1.0",
		"vite-plugin-rewrite-all": "^1.0.0",
		"typescript": "4.5.4",
		"vite": "2.7.13"
	},
	"dependencies": {
		"react": "^17.0.2",
		"react-dom": "^17.0.2"
	}
}`)),
	},
	{
		filename: "index.html",
		tmpl: template.Must(template.New("index.html").Parse(`<!DOCTYPE html>
<html lang="en">
	<head>
		<meta charset="utf-8" />
		<title>Welcome to Namespace</title>
	</head>
	<body>
		<div id="app"></div>
		<script type="module" src="./src/index.tsx"></script>
	</body>
</html>`)),
	},
	{
		filename: "tsconfig.json",
		tmpl: template.Must(template.New("tsconfig").Parse(`{
	"compilerOptions": {
		"target": "ESNext",
		"useDefineForClassFields": true,
		"lib": [
				"DOM",
				"DOM.Iterable",
				"ESNext"
		],
		"allowJs": false,
		"skipLibCheck": false,
		"esModuleInterop": false,
		"allowSyntheticDefaultImports": true,
		"strict": true,
		"forceConsistentCasingInFileNames": true,
		"module": "ESNext",
		"moduleResolution": "Node",
		"resolveJsonModule": true,
		"isolatedModules": true,
		"noEmit": true,
		"jsx": "react-jsx",
		"noImplicitAny": false,
	},
	"include": [
		"./src"
	]
}`)),
	},
}
