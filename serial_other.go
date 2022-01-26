//go:build !linux
// +build !linux

package serial

import (
	"fmt"
	"runtime"
)

func Open(name string, baud int) (Port, error) {
	return nil, fmt.Errorf("not implemented on %s OS", runtime.GOOS)
}
