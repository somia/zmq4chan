// +build linux,386 linux,arm

package zmq4chan

import (
	"syscall"
)

func setEpollDataPtr(e *syscall.EpollEvent, ptr uintptr) {
	e.Fd = int32(ptr)
}

func getEpollDataPtr(e *syscall.EpollEvent) uintptr {
	return uintptr(e.Fd)
}
