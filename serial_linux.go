//go:build linux
// +build linux

package serial

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// Open opens a serial port with the specified name (like, /dev/ttyUSB0) and baud rate.
// It will create a raw, local, 8N1 serial connection.
func Open(name string, baud int) (Port, error) {
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
	if baud == 250000 {
		var ss serial_struct
		fmt.Fprintf(os.Stderr, "sizeof(ss): %d\n", unsafe.Sizeof(ss))
		if err = ioctlSS(f.Fd(), syscall.TIOCGSERIAL, &ss); err != nil {
			return nil, fmt.Errorf("failed to request serial_struct: %v", err)
		}
		ss.flags &= ^ASYNC_SPD_MASK
		ss.flags |= ASYNC_SPD_CUST
		ss.custom_divisor = uint32((int(ss.baud_base) + (baud / 2)) / baud)
		if ss.custom_divisor < 1 {
			ss.custom_divisor = 1
		}
		if err = ioctlSS(f.Fd(), syscall.TIOCSSERIAL, &ss); err != nil {
			return nil, fmt.Errorf("failed to set custom baud rate: %v", err)
		}
		if err = ioctlSS(f.Fd(), syscall.TIOCSSERIAL, &ss); err != nil {
			return nil, fmt.Errorf("failed to set custom baud rate (second pass): %v", err)
		}
		if err = tio.setSpeed(B38400); err != nil {
			return nil, err
		}
	} else {
		br, err := convRate(baud)
		if err != nil {
			return nil, err
		}

		if err = tio.setSpeed(br); err != nil {
			return nil, err
		}
	}
	if err = tio.apply(f.Fd()); err != nil {
		return nil, err
	}
	tio2, err := query(f.Fd())
	if err != nil {
		return nil, fmt.Errorf("failed to query serial attributes: %v", err)
	}
	if tio.speed() != tio2.speed() && baud != 250000 {
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

var knownRates = map[int]uint32{
	50:      B50,
	75:      B75,
	110:     B110,
	134:     B134,
	150:     B150,
	200:     B200,
	300:     B300,
	600:     B600,
	1200:    B1200,
	1800:    B1800,
	2400:    B2400,
	4800:    B4800,
	9600:    B9600,
	19200:   B19200,
	38400:   B38400,
	57600:   B57600,
	115200:  B115200,
	230400:  B230400,
	460800:  B460800,
	500000:  B500000,
	576000:  B576000,
	921600:  B921600,
	1000000: B1000000,
	1152000: B1152000,
	1500000: B1500000,
	2000000: B2000000,
	2500000: B2500000,
	3000000: B3000000,
	3500000: B3500000,
	4000000: B4000000,
}

// convRate converts numerical rate into the baud rate code, like B115200.
func convRate(baud int) (uint32, error) {
	v, ok := knownRates[baud]
	if !ok {
		return 0, fmt.Errorf("unsupported baud rate: %v", baud)
	}
	return v, nil
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

type serial_struct struct {
	typ             uint32
	line            uint32
	port            uint32
	irq             uint32
	flags           int32
	xmit_fifo_size  uint32
	custom_divisor  uint32
	baud_base       uint32
	close_delay     uint16
	io_type         byte
	reserved_char   byte
	hub6            int
	closing_wait    uint16
	closing_wait2   uint16
	iomem_base      uintptr
	iomem_reg_shift uint16
	port_high       uint32
	iomap_base      int64
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
	if err := ioctl(fd, TCSETSF, tio); err != nil {
		return err
	}
	//if err := fcntl(fd, syscall.F_SETFL, 0); err != nil {
	//	return err
	//}
	return nil
}

// query gets serial attributes from the fd.
func query(fd uintptr) (*termios, error) {
	tio := new(termios)
	if err := ioctl(fd, TCGETS, tio); err != nil {
		return nil, err
	}
	return tio, nil
}

func rawFcntl(fd uintptr, cmd int, arg uintptr) error {
	_, _, err := syscall.RawSyscall(syscall.SYS_FCNTL, fd, uintptr(cmd), arg)
	if err != 0 {
		return err
	}
	return nil
}

func fcntl(fd uintptr, cmd int, arg int) error {
	return rawFcntl(fd, cmd, uintptr(arg))
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

func ioctlSS(fd uintptr, req uint, ss *serial_struct) error {
	return rawIoctl(fd, req, uintptr(unsafe.Pointer(ss)))
}

const (
	ASYNCB_SPD_HI  = 4  /* Use 57600 instead of 38400 bps */
	ASYNCB_SPD_VHI = 5  /* Use 115200 instead of 38400 bps */
	ASYNCB_SPD_SHI = 12 /* Use 230400 instead of 38400 bps */
	ASYNC_SPD_HI   = (1 << ASYNCB_SPD_HI)
	ASYNC_SPD_SHI  = (1 << ASYNCB_SPD_SHI)
	ASYNC_SPD_VHI  = (1 << ASYNCB_SPD_VHI)
	ASYNC_SPD_CUST = (ASYNC_SPD_HI | ASYNC_SPD_VHI)
	ASYNC_SPD_MASK = (ASYNC_SPD_HI | ASYNC_SPD_VHI | ASYNC_SPD_SHI)
)
