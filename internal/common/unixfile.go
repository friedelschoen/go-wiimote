package common

import (
	"io"
	"syscall"
)

type UnbufferedFile int

func (fd UnbufferedFile) Read(b []byte) (int, error) {
	return syscall.Read(int(fd), b)
}

func (fd UnbufferedFile) Write(b []byte) (int, error) {
	return syscall.Write(int(fd), b)
}

func (fd UnbufferedFile) Seek(offset int64, whence int) (int64, error) {
	return syscall.Seek(int(fd), offset, whence)
}

func (fd UnbufferedFile) ReadAt(buf []byte, off int64) (n int, err error) {
	if _, err := fd.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}
	return fd.Read(buf)
}

func (fd UnbufferedFile) Close() error {
	return syscall.Close(int(fd))
}
