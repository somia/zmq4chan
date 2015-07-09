// +build linux,amd64 linux,arm64 linux,ppc64le

package zmq4chan

import (
	"syscall"
)

func setEpollDataPtr(e *syscall.EpollEvent, ptr uintptr) {
	e.Fd = int32(ptr & 0xffffffff)
	e.Pad = int32(ptr >> 32)
}

func getEpollDataPtr(e *syscall.EpollEvent) uintptr {
	return uintptr(e.Fd) | uintptr(e.Pad)<<32
}
