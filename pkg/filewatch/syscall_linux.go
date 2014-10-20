package filewatch

import (
	"fmt"
	"syscall"
)

func Getrlimit(rlimit *Rlimit) error {
	r := new(syscall.Rlimit)
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, r)
	if err == nil {
		rlimit.Cur = uint64(r.Cur)
		rlimit.Max = uint64(r.Max)
		fmt.Println("Current system's limits files open per process to ",
			rlimit.Max, rlimit.Cur)
	}
	return err
}
