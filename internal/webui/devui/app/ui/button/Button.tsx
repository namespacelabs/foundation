// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import classNames from "classnames";
import React, { MouseEventHandler } from "react";
import * as style from "./style.module.css";

export default function Button(props: {
	children: React.ReactNode;
	onClick?: MouseEventHandler;
	compact?: boolean;
	title?: string;
}) {
	return (
		<button
			onClick={props.onClick}
			title={props.title}
			className={classNames(style.btn, { [style.compact]: props.compact })}>
			{props.children}
		</button>
	);
}
