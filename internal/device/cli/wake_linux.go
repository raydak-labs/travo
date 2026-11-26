package cli

import "syscall"

func wakeSocketOptions(fd uintptr, bridge string) error {
	if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1); err != nil {
		return err
	}
	return syscall.BindToDevice(int(fd), bridge)
}
