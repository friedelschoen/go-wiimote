package driver

import (
	"iter"
	"os"
	"syscall"
	"time"

	"github.com/friedelschoen/go-wiimote"
	"github.com/friedelschoen/go-wiimote/internal/common"
	"github.com/friedelschoen/go-wiimote/internal/sequences"
)

// IterDevices returns all currently available devices. It returns an error if the
// initialization failed. Each iteration yields a device and error if the device-creation failed.
func IterDevices() (iter.Seq[wiimote.DeviceInfo], error) {
	enum := NewEnumerate()
	if err := enum.AddMatchSubsystem("hid"); err != nil {
		return nil, err
	}

	iter, err := enum.Devices()
	if err != nil {
		return nil, err
	}

	deviter := sequences.Map(iter, func(dev wiimote.DeviceInfo) wiimote.DeviceInfo {
		if dev == nil {
			return nil
		}
		if dev.Action() != "" && dev.Action() != "add" {
			return nil
		}
		if dev.Driver() != "wiimote" || dev.Subsystem() != "hid" {
			return nil
		}
		return dev
	})
	deviter = sequences.Filter(deviter, func(d wiimote.DeviceInfo) bool {
		return d != nil
	})
	return deviter, nil
}

// WiimoteMonitor describes a monitor for wiimote-devices. This includes currently available
// but also hot-plugged devices.
//
// Monitors are not thread-safe.
type WiimoteMonitor struct {
	wiimote.Poller[wiimote.DeviceInfo]

	monitor wiimote.DeviceMonitor
	enum    chan wiimote.DeviceInfo
}

// NewWiimoteMonitor creates a new monitor.
//
// A monitor always provides all devices that are available on a system
// and hot-plugged devices.
//
// The object and underlying structure is freed automatically by default.
func NewWiimoteMonitor() (*WiimoteMonitor, error) {
	var mon WiimoteMonitor
	mon.Poller = common.NewPoller(&mon)

	devs, err := IterDevices()
	if err != nil {
		return nil, err
	}
	mon.enum = make(chan wiimote.DeviceInfo)
	go func() {
		for dev := range devs {
			mon.enum <- dev
		}
		close(mon.enum)
	}()

	mon.monitor = NewMonitor()
	if mon.monitor == nil {
		return nil, os.ErrInvalid
	}
	if err := mon.monitor.FilterAddMatchSubsystem("hid"); err != nil {
		return nil, err
	}
	if err := mon.monitor.EnableReceiving(); err != nil {
		return nil, err
	}
	return &mon, nil
}

// FD returns the file-descriptor to notify readiness. The FD is non-blocking.
// Only one file-descriptor exists, that is, this function always returns the
// same descriptor.
func (mon *WiimoteMonitor) FD() int {
	fd := mon.monitor.FD()
	syscall.SetNonblock(fd, true)
	return fd
}

// Poll returns a single device-name on each call. A device-name is actually
// an absolute sysfs path to the device's root-node. This is normally a path
// to /sys/bus/hid/devices/[dev]/. You can use this path to create a new
// struct wii_iface object.
//
// After a monitor was created, this function returns all currently available
// devices. After all devices have been returned. After that, this function polls the
// monitor for hotplug events and returns hotplugged devices,
// if the monitor was opened to watch the system for hotplug events.
//
// Use FD() to get notified when a new event is available.
func (mon *WiimoteMonitor) Poll() (wiimote.DeviceInfo, bool, error) {
	// test if enumerator has devices, then wait for new devices
	if iter, ok := <-mon.enum; ok {
		return iter, true, nil
	}

	dev := mon.monitor.ReceiveDevice()
	if dev == nil {
		return nil, false, common.ErrPollAgain
	}
	if (dev.Action() != "" && dev.Action() != "add") || dev.Driver() != "wiimote" || dev.Subsystem() != "hid" {
		return nil, false, common.ErrPollAgain
	}
	time.Sleep(50 * time.Millisecond)
	return dev, false, nil
}
