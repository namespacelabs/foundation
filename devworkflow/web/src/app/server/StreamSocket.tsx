// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import React from "react";
import { StreamOutput } from "../../ui/terminal/StreamOutput";
import { OutputSocket } from "../../devworkflow/output";

export default function StreamSocket(props: { what: string }) {
  return (
    <StreamOutput
      makeSocket={() =>
        new OutputSocket({
          endpoint: props.what,
        })
      }
    />
  );
}