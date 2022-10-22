// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

export type Segment = {
	t: string;
	k: string;
};

export function renderParts(parts: Segment[]) {
	return parts?.map((p) => {
		if (p.k === "image") {
			let parts = p.t.split("@");
			return (
				<a title={p.t} href={"https://" + parts[0]}>
					🌐{parts[0]}
				</a>
			);
		}

		return <span>{p.t}</span>;
	});
}
