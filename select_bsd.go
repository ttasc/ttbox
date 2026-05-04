//go:build darwin || freebsd || netbsd || openbsd || dragonfly

package ttbox

import "syscall"

func selectRead(maxFd int, set *syscall.FdSet, tv *syscall.Timeval) (int, error) {
	err := syscall.Select(maxFd+1, set, nil, nil, tv)
	if err != nil {
		return 0, err
	}
	return 1, nil
}
