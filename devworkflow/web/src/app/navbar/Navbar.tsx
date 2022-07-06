// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { Link } from "wouter";
import { LogoIcon } from "@namespacelabs.dev/webui-components/logo/Logo";
import classes from "./navbar.module.css";

export function Navbar() {
	return (
		<div className={classes.fixedNavbar}>
			<Link href="/">
				<a>
					<LogoIcon filled />
				</a>
			</Link>
			<div className={classes.navbarMain}>Development UI</div>
		</div>
	);
}
