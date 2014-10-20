package filewatch

import (
	"fmt"
)

func Getrlimit(rlimit *Rlimit) error {
	rlimit.Cur = 1 << 10
	rlimit.Max = 1 << 10
	fmt.Println("FAKE Windows limits files open per process to ",
		rlimit.Max, rlimit.Cur)
	return nil
}
