// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package web

import "text/template"

type serviceTmplOptions struct {
	BackendPkgs []string
}

type templateFile struct {
	filename string
	tmpl     *template.Template
}

var templates = []templateFile{
	{
		filename: "src/index.tsx",
		tmpl: template.Must(template.New("index.tsx").Parse(`
import React from "react";
import ReactDOM from "react-dom";

ReactDOM.render(
	<React.StrictMode>
			<App />
	</React.StrictMode>,
	document.getElementById("app")
);

function App() {
	return (
		<>
			Hello, world!
		</>
	);
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
	},
	"include": [
		"./src"
	]
}`)),
	},
}
