// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import classNames from "classnames";
import { Link } from "wouter";
import { useData } from "../../datamodel/StackObserver";
import { ExternalLinkIcon } from "../../icons";
import classes from "./server.module.css";

export function PackageLink(props: {
	label?: string;
	packageName: string;
	secondary?: boolean;
	className?: string;
	collapsable?: boolean;
}) {
	let parsed = parsePackage(props.packageName);
	let url = "";

	if (parsed.kind === "parsed" && parsed.domain === "github.com") {
		// XXX `main` is not universal. https://namespacelabs.dev/foundation/issues/50
		url = `https://github.com/${parsed.repo.join("/")}/tree/main/${parsed.content.join("/")}`;
	}

	return (
		<ParsedPackageLink
			href={url}
			label={props.label}
			secondary={props.secondary}
			className={props.className}
			collapsable={props.collapsable}
			parsed={parsed}
		/>
	);
}

export function ServerLink(props: { packageName: string }) {
	const data = useData();

	let url = "";
	for (const s of data?.stack?.entry || []) {
		if (s.server.package_name == props.packageName) {
			url = `/server/${s.server.id}`;
		}
	}

	let parsed = parsePackage(props.packageName);
	return <ParsedPackageLink parsed={parsed} collapsable={true} href={url} />;
}

function ParsedPackageLink(props: {
	label?: string;
	parsed: Unparsed | ParsedPackage;
	secondary?: boolean;
	className?: string;
	collapsable?: boolean;
	href?: string;
}) {
	let prefix: string = "";
	let labelParts: string[];

	if (props.parsed.kind === "unparsed") {
		labelParts = props.parsed.parts || [];
	} else {
		let { service, domain, repo, content } = props.parsed;

		if (props.collapsable) {
			prefix = service + "/" + repo.map((p) => p[0]).join("/");
			labelParts = content || [];
		} else {
			labelParts = [domain].concat(repo).concat(content);
		}
	}

	let external = !props.href?.startsWith("/");

	let body = (
		<a
			href={external ? props.href : undefined}
			target="_blank"
			className={classNames(classes.dep, props.className, {
				[classes.secondary]: props.secondary,
			})}>
			{props.label ?? (
				<>
					{prefix ? <span className={classes.prefix}>{prefix}</span> : null}
					<PackageName>{labelParts}</PackageName>
				</>
			)}
			{external ? <ExternalLinkIcon /> : null}
		</a>
	);

	if (external || !props.href) {
		return body;
	}

	return <Link href={props.href}>{body}</Link>;
}

type Unparsed = {
	kind: "unparsed";

	parts?: string[];
};

type ParsedPackage = {
	kind: "parsed";

	service: string;
	domain: string;
	repo: string[];
	content: string[];
};

function parsePackage(packageName: string): Unparsed | ParsedPackage {
	let parts = packageName.split("/");
	if (parts.length < 4 || parts[0] != "github.com") {
		return {
			kind: "unparsed",
			parts,
		};
	}

	let content = parts.splice(3);
	let repo = parts.splice(1);

	return {
		kind: "parsed",
		service: "gh",
		domain: parts[0],
		repo,
		content,
	};
}

function PackageName(props: { children: string[] }) {
	return (
		<>
			{props.children.map((p, k) => (
				<span key={k} className={classes.p}>{`${p}${
					k < props.children.length - 1 ? "/" : ""
				}`}</span>
			))}
		</>
	);
}
