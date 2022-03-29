//go:build darwin
// +build darwin

package disk

import "syscall"

func FSType(path string) (string, error) {
	return "unknown", nil
}
