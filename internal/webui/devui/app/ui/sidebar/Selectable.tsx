// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import classNames from "classnames";
import React from "react";
import { Link } from "wouter";
import classes from "./sidebar.module.css";

export default function Selectable(props: { selected: boolean; children: React.ReactNode }) {
	return (
		<div
			className={classNames(classes.selectable, {
				[classes.selected]: props.selected,
			})}>
			{props.children}
		</div>
	);
}
