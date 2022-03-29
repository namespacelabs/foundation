// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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