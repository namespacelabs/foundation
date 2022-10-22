// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import { useData } from "../../datamodel/StackObserver";
import { Sidebar } from "../../ui/sidebar/Sidebar";
import ServerBlock from "./ServerBlock";

export default function AppSidebar(props: { fixed?: boolean; footer?: JSX.Element }) {
	const data = useData();

	return <Sidebar {...props}>{data?.stack ? <ServerBlock data={data} /> : null}</Sidebar>;
}
