package xwiimote

import "syscall"

type SysFile int

func (fd SysFile) Read(b []byte) (int, error) {
	return syscall.Read(int(fd), b)
}

func (fd SysFile) Write(b []byte) (int, error) {
	return syscall.Write(int(fd), b)
}

func (fd SysFile) Close() error {
	return syscall.Close(int(fd))
}
