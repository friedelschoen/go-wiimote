//go:build linux

package driver

import (
	"github.com/friedelschoen/go-wiimote"
	"github.com/friedelschoen/go-wiimote/driver/udev"
)

func NewEnumerate() wiimote.DeviceEnumerator {
	return udev.NewEnumerate()
}

func NewMonitor() wiimote.DeviceMonitor {
	return udev.NewMonitorFromNetlink(udev.MonitorUdev)
}
