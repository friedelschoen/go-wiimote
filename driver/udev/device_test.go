package udev

import (
	"runtime"
	"testing"
)

func TestDeviceZero(t *testing.T) {
	d := NewDeviceFromDeviceID("c1:5")
	if d.Subsystem() != "mem" {
		t.Fail()
	}
	if d.Sysname() != "zero" {
		t.Fail()
	}
	if d.Syspath() != "/sys/devices/virtual/mem/zero" {
		t.Fail()
	}
	if d.Devnode() != "/dev/zero" {
		t.Fail()
	}
	if d.SysattrValue("subsystem") != "mem" {
		t.Fail()
	}
}

func TestDeviceGC(t *testing.T) {
	runtime.GC()
}
