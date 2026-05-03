//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package ttbox

import (
	"syscall"
	"unsafe"
)

func setFd(set *syscall.FdSet, fd int) {
	if fd < 0 {
		return
	}
	bytes := unsafe.Slice((*byte)(unsafe.Pointer(set)), int(unsafe.Sizeof(*set)))
	idx := fd / 8
	if idx >= len(bytes) {
		return
	}
	bytes[idx] |= 1 << (uint(fd) % 8)
}

func fdIsSet(set *syscall.FdSet, fd int) bool {
	if fd < 0 {
		return false
	}
	bytes := unsafe.Slice((*byte)(unsafe.Pointer(set)), int(unsafe.Sizeof(*set)))
	idx := fd / 8
	if idx >= len(bytes) {
		return false
	}
	return bytes[idx]&(1<<(uint(fd)%8)) != 0
}
