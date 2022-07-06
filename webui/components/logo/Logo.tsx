// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import React from "react";
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
	const logoSvg = props?.filled ? (
		<svg viewBox="0 0 67 67" fill="none" xmlns="http://www.w3.org/2000/svg">
			<path
				d="M33.8872 66.5206C52.109 66.5206 66.8807 51.7489 66.8807 33.5271C66.8807 15.3054 52.109 0.533691 33.8872 0.533691C15.6655 0.533691 0.893799 15.3054 0.893799 33.5271C0.893799 51.7489 15.6655 66.5206 33.8872 66.5206Z"
				fill="#1C32FF"
			/>
			<path d="M51.0722 51.8223V46.8785H34.5089V51.8223H51.0722Z" fill="white" />
			<path
				d="M38.5944 43.3556H34.5386L22.8895 26.9255V43.3556H17.9457V18.6957H22.7711L33.6505 34.2821V18.6957H38.5944V43.3556Z"
				fill="white"
			/>
		</svg>
	) : (
		<svg viewBox="0 0 65 66" fill="none" xmlns="http://www.w3.org/2000/svg">
			<path d="M48.3054 50.045V45.4892H33.0423V50.045H48.3054Z" fill="#1C32FF" />
			<path
				d="M36.8479 42.2429H33.0697L22.3351 27.1027V42.2429H17.7521V19.5189H22.1987L32.2513 33.8817V19.5189H36.8479V42.2429Z"
				fill="#1C32FF"
			/>
			<path
				d="M32.4695 65.649C26.0489 65.649 19.7726 63.7451 14.4341 60.178C9.09558 56.6109 4.93474 51.5409 2.4777 45.6091C0.0206583 39.6773 -0.622216 33.1501 0.630371 26.8529C1.88296 20.5557 4.97475 14.7714 9.51477 10.2314C14.0548 5.69136 19.8391 2.59957 26.1363 1.34698C32.4335 0.0943974 38.9607 0.737273 44.8925 3.19431C50.8243 5.65135 55.8943 9.8122 59.4614 15.1507C63.0284 20.4892 64.9324 26.7655 64.9324 33.1861C64.9251 41.7936 61.5026 50.0464 55.4162 56.1328C49.3298 62.2192 41.077 65.6418 32.4695 65.649ZM32.4695 4.82882C26.8577 4.82882 21.3719 6.49309 16.706 9.61113C12.0402 12.7292 8.4038 17.1609 6.25687 22.3458C4.10994 27.5307 3.5489 33.2359 4.64471 38.7397C5.74051 44.2434 8.44393 49.2986 12.413 53.2658C16.3822 57.233 21.4386 59.934 26.943 61.0272C32.4473 62.1203 38.1521 61.5566 43.336 59.4071C48.5199 57.2577 52.9499 53.6192 56.0657 48.9519C59.1815 44.2845 60.8431 38.7979 60.8404 33.1861C60.8296 25.6662 57.8366 18.4577 52.518 13.1416C47.1993 7.82555 39.9893 4.83604 32.4695 4.82882Z"
				fill="#1C32FF"
			/>
		</svg>
	);
	return <div className={classes.namespaceLabsImg}>{logoSvg}</div>;
}
