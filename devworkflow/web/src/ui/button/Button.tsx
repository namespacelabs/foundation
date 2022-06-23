// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import classNames from "classnames";
import React, { MouseEventHandler } from "react";
import * as style from "./style.module.css";

export default function Button(props: {
	children: React.ReactNode;
	onClick?: MouseEventHandler;
	compact?: boolean;
}) {
	return (
		<button
			onClick={props.onClick}
			className={classNames(style.btn, { [style.compact]: props.compact })}>
			{props.children}
		</button>
	);
}
