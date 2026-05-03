//go:build darwin || freebsd || netbsd || openbsd || dragonfly

package ttbox

import "syscall"

func selectRead(fd int, set *syscall.FdSet, tv *syscall.Timeval) (int, error) {
	err := syscall.Select(fd+1, set, nil, nil, tv)
	if err != nil {
		return 0, err
	}
	if fdIsSet(set, fd) {
		return 1, nil
	}
	return 0, nil
}
