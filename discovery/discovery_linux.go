//go:build linux

package discovery

import (
	"github.com/friedelschoen/go-wiimote"
	"github.com/friedelschoen/go-wiimote/internal/udev"
)

func NewEnumerate() wiimote.DeviceEnumerator {
	return udev.NewEnumerate()
}

func NewMonitor() wiimote.DeviceMonitor {
	return udev.NewMonitorFromNetlink(udev.MonitorUdev)
}
