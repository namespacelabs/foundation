// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import React from "react";
import classes from "./panel.module.css";

export default function Panel(props: { children: React.ReactNode }) {
  return <div className={classes.panel}>{props.children}</div>;
}