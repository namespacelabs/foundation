// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import React from "react";
import ReactDOM from "react-dom";
import App from "@namespacelabs.dev/webui-devui-app/app/App";
import "./index.css";

ReactDOM.render(
	<React.StrictMode>
		<App />
	</React.StrictMode>,
	document.getElementById("app")
);
