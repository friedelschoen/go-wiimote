//go:build linux

package driver

import (
	"errors"
	"fmt"

	"github.com/friedelschoen/go-udev"
	"github.com/friedelschoen/go-wiimote"
	"github.com/friedelschoen/go-wiimote/driver/commonhid"
	"github.com/friedelschoen/go-wiimote/driver/linuxhidraw"
	"github.com/friedelschoen/go-wiimote/driver/linuxkernel"
)

func NewEnumerate() wiimote.DeviceEnumerator {
	return udev.NewEnumerate()
}

func NewMonitor() wiimote.DeviceMonitor {
	return udev.NewMonitorFromNetlink(udev.MonitorUdev)
}

func NewDevice(info wiimote.DeviceInfo, backend Backend) (wiimote.Device, error) {
	switch backend {
	case BackendKernel:
		return linuxkernel.NewDevice(info, NewMonitor, NewEnumerate)
	case BackendHID:
		transport, err := linuxhidraw.NewTransportFromInfo(info)
		if err != nil {
			return nil, fmt.Errorf("unable to open device: %w", err)
		}
		return commonhid.NewDevice(transport), nil
	default:
		return nil, errors.New("unknown backend")
	}
}
