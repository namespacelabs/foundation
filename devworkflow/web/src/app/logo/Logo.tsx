// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import classes from "./logo.module.css";

export function Logo() {
	return (
		<a className={classes.logo} href="https://namespacelabs.com/" target="_blank">
			<div className={classes.attribution}>
				<div className={classes.namespaceLabs}>
					<LogoIcon />
					<div className={classes.namespaceLabsText}>
						<span>namespace</span>
						<span>labs</span>
					</div>
				</div>
			</div>
		</a>
	);
}

export function LogoIcon(props: { filled?: boolean }) {
	// First argument of `new URL` must be static.
	const logoURL = props?.filled
		? new URL("/icons/logo_filled.svg", import.meta.url)
		: new URL("/icons/logo.svg", import.meta.url);
	return <img className={classes.namespaceLabsImg} src={logoURL.toString()}></img>;
}
