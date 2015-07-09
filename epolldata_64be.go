// +build linux,ppc64

package zmq4chan

import (
	"syscall"
)

func setEpollDataPtr(e *syscall.EpollEvent, ptr uintptr) {
	e.Fd = int32(ptr >> 32)
	e.Pad = int32(ptr & 0xffffffff)
}

func getEpollDataPtr(e *syscall.EpollEvent) uintptr {
	return uintptr(e.Fd)<<32 | uintptr(e.Pad)
}
