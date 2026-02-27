//go:build linux

package backend

import (
	"github.com/friedelschoen/go-wiimote"
	"github.com/friedelschoen/go-wiimote/driver/linuxhid"
)

func NewDevice(info wiimote.DeviceInfo, backend Backend) (wiimote.Device, error) {
	switch backend {
	case BackendKernel:
		return linuxhid.NewDevice(info)
	default:
		panic("backend not supported")
	}
}
