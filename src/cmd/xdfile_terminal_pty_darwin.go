//go:build darwin

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

type xdfileDarwinPTYBackend struct {
	master *os.File
}

func (b *xdfileDarwinPTYBackend) Read(p []byte) (int, error) {
	return b.master.Read(p)
}

func (b *xdfileDarwinPTYBackend) Write(p []byte) (int, error) {
	return b.master.Write(p)
}

func (b *xdfileDarwinPTYBackend) Close() error {
	if b == nil || b.master == nil {
		return nil
	}
	return b.master.Close()
}

func (b *xdfileDarwinPTYBackend) Resize(width int, height int) error {
	if b == nil || b.master == nil {
		return nil
	}
	return xdfileDarwinSetPTYSize(int(b.master.Fd()), width, height)
}

func xdfileStartCommandPTYBackend(
	dir string,
	path string,
	args []string,
	width int,
	height int,
) (xdfileTerminalPTYBackend, *os.Process, error) {
	master, slave, err := xdfileDarwinOpenPTY(width, height)
	if err != nil {
		return nil, nil, err
	}

	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	cmd.Env = xdfileCommandExecutionEnvironment(os.Environ())
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0,
	}

	if err := cmd.Start(); err != nil {
		_ = slave.Close()
		_ = master.Close()
		return nil, nil, fmt.Errorf("spawn PTY command: %w", err)
	}
	_ = slave.Close()

	return &xdfileDarwinPTYBackend{master: master}, cmd.Process, nil
}

func xdfileDarwinOpenPTY(width int, height int) (*os.File, *os.File, error) {
	letters := "pqrstuvwxyzPQRST"
	digits := "0123456789abcdef"
	var lastErr error
	for _, letter := range letters {
		for _, digit := range digits {
			masterPath := fmt.Sprintf("/dev/pty%c%c", letter, digit)
			slavePath := fmt.Sprintf("/dev/tty%c%c", letter, digit)
			master, err := os.OpenFile(masterPath, os.O_RDWR|syscall.O_NOCTTY, 0)
			if err != nil {
				lastErr = err
				continue
			}
			slave, err := os.OpenFile(slavePath, os.O_RDWR|syscall.O_NOCTTY, 0)
			if err != nil {
				lastErr = err
				_ = master.Close()
				continue
			}
			if err := xdfileDarwinSetPTYSize(int(master.Fd()), width, height); err != nil {
				_ = slave.Close()
				_ = master.Close()
				return nil, nil, err
			}
			return master, slave, nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no BSD PTY devices found")
	}
	return nil, nil, fmt.Errorf("open Darwin PTY: %w", lastErr)
}

func xdfileDarwinSetPTYSize(fd int, width int, height int) error {
	size := &unix.Winsize{
		Col: uint16(max(1, width)),
		Row: uint16(max(1, height)),
	}
	if err := unix.IoctlSetWinsize(fd, unix.TIOCSWINSZ, size); err != nil {
		return fmt.Errorf("resize PTY: %w", err)
	}
	return nil
}
