//go:build linux

package driver

import (
	"github.com/friedelschoen/go-wiimote"
	"github.com/friedelschoen/go-wiimote/driver/linuxkernel"
)

func NewDevice(info wiimote.DeviceInfo, backend Backend) (wiimote.Device, error) {
	switch backend {
	case BackendKernel:
		return linuxkernel.NewDevice(info, NewMonitor, NewEnumerate)
	default:
		panic("backend not supported")
	}
}
