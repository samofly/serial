package serial

import "io"

// Port describes an opened serial port.
type Port interface {
	io.ReadWriteCloser
}
