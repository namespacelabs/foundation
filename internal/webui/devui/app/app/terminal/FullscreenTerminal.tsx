// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import ServerTerminal from "../server/ServerTerminal";

export default function FullscreenTerminal(props: { id: string }) {
	return <ServerTerminal serverId={props.id} what="terminal" wireStdin={true} />;
}
