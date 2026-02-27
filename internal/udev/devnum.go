package udev

// #cgo pkg-config: libudev
// #include <libudev.h>
// #include <linux/types.h>
// #include <linux/kdev_t.h>
//
// int go_udev_major(dev_t d) {
//   return MAJOR(d);
// }
// int go_udev_minor(dev_t d) {
//   return MINOR(d);
// }
// dev_t go_udev_mkdev(int major, int minor) {
//   return MKDEV(major, minor);
// }
import "C"

// devnum is a kernel device number
type devnum struct {
	d C.dev_t
}

// Major returns the major part of a Devnum
func (d devnum) Major() int {
	return int(C.go_udev_major(d.d))
}

// Minor returns the minor part of a Devnum
func (d devnum) Minor() int {
	return int(C.go_udev_minor(d.d))
}

// mkDev creates a Devnum from a major and minor number
func mkDev(major, minor int) devnum {
	return devnum{C.go_udev_mkdev((C.int)(major), (C.int)(minor))}
}
