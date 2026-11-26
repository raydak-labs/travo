//go:build !linux

package cli

import "syscall"

// Development hosts only: no SO_BINDTODEVICE outside Linux; the route picks the interface.
func wakeSocketOptions(fd uintptr, _ string) error {
	return syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
}
