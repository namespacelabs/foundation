// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package filewatcher

import "namespacelabs.dev/go-filenotify"

func NewWatcher() (filenotify.FileWatcher, error) {
	return filenotify.NewPollingWatcher(), nil
}
