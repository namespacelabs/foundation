// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import { useRoute } from "wouter";

export function useTasksRoute() {
	return useRoute("/tasks/:id?/:what?");
}
