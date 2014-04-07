package serial

import (
	"fmt"
	"io"
	"os"
	"syscall"
	"unsafe"
)

// Port describes an opened serial port.
type Port interface {
	io.ReadWriteCloser
}

// Open opens a serial port with the specified name (like, /dev/ttyUSB0) and baud rate.
// It will create a raw, local, 8N1 serial connection.
func Open(name string, baud int) (Port, error) {
	br, err := convRate(baud)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(name, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			f.Close()
		}
	}()

	tio := newRaw()

	if err = tio.setSpeed(br); err != nil {
		return nil, err
	}
	if err = tio.apply(f.Fd()); err != nil {
		return nil, err
	}
	tio2, err := query(f.Fd())
	if err != nil {
		return nil, fmt.Errorf("failed to query serial attributes: %v", err)
	}
	if tio.speed() != tio2.speed() {
		return nil, fmt.Errorf("failed to set baud rate. Want: %d, got: %d", tio.speed(), tio2.speed())
	}

	return &port{f: f}, nil
}

// port represents an opened serial connection.
type port struct {
	f *os.File
}

// Read implements io.Reader
func (p *port) Read(buf []byte) (int, error) { return p.f.Read(buf) }

// Write implements io.Writer
func (p *port) Write(buf []byte) (int, error) { return p.f.Write(buf) }

// Close implements io.Closer
func (p *port) Close() error { return p.f.Close() }

// convRate converts numerical rate into the baud rate code, like B115200.
func convRate(baud int) (uint32, error) {
	switch baud {
	case 115200:
		return B115200, nil
	default:
		return 0, fmt.Errorf("unsupported baud rate: %v", baud)
	}
}

// termios is a low-level structure that Linux kernel will understand.
type termios struct {
	iflag   uint32
	oflag   uint32
	cflag   uint32
	lflag   uint32
	line    byte
	cc      [32]byte
	unused0 uint32
	unused1 uint32
}

func newRaw() *termios {
	return &termios{
		cflag: CS8 | CLOCAL | CREAD | HUPCL,
		cc:    [32]byte{VMIN: 1, VTIME: 0},
	}
}

func (tio *termios) setSpeed(baud uint32) error {
	if (baud & ^uint32(CBAUD)) != 0 {
		return fmt.Errorf("setSpeed: baud=%0x, does not fit to mask: %0x", baud, CBAUD)
	}
	tio.cflag &= ^uint32(CBAUD)
	tio.cflag |= baud
	return nil
}

func (tio *termios) speed() uint32 {
	return tio.cflag & CBAUD
}

// apply sets serial attributes to the fd.
func (tio *termios) apply(fd uintptr) error {
	// TODO(krasin): may be also support TCSETSW
	return ioctl(fd, TCSETSF, tio)
}

// query gets serial attributes from the fd.
func query(fd uintptr) (*termios, error) {
	tio := new(termios)
	if err := ioctl(fd, TCGETS, tio); err != nil {
		return nil, err
	}
	return tio, nil
}

func rawIoctl(fd uintptr, req uint, arg uintptr) error {
	_, _, err := syscall.RawSyscall(syscall.SYS_IOCTL, fd, uintptr(req), arg)
	if err != 0 {
		return err
	}
	return nil
}

func ioctl(fd uintptr, req uint, tio *termios) error {
	return rawIoctl(fd, req, uintptr(unsafe.Pointer(tio)))
}
