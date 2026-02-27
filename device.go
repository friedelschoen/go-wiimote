package wiimote

import "iter"

// Devnum is a kernel device number
type Devnum interface {
	// Major returns the major part of a Devnum
	Major() int

	// Minor returns the minor part of a Devnum
	Minor() int
}

type DeviceInfo interface {
	// Parent returns the parent Device, or nil if the receiver has no parent Device
	Parent() DeviceInfo

	// ParentWithSubsystemDevtype returns the parent Device with the given subsystem and devtype,
	// or nil if the receiver has no such parent device
	ParentWithSubsystemDevtype(subsystem, devtype string) DeviceInfo

	// Devpath returns the kernel devpath value of the udev device.
	// The path does not contain the sys mount point, and starts with a '/'.
	Devpath() string

	// Subsystem returns the subsystem string of the udev device.
	// The string does not contain any "/".
	Subsystem() string

	// Devtype returns the devtype string of the udev device.
	Devtype() string

	// Sysname returns the sysname of the udev device (e.g. ttyS3, sda1...).
	Sysname() string

	// Syspath returns the sys path of the udev device.
	// The path is an absolute path and starts with the sys mount point.
	Syspath() string

	// Sysnum returns the trailing number of of the device name
	Sysnum() string

	// Devnode returns the device node file name belonging to the udev device.
	// The path is an absolute path, and starts with the device directory.
	Devnode() string

	// IsInitialized checks if udev has already handled the device and has set up
	// device node permissions and context, or has renamed a network device.
	//
	// This is only implemented for devices with a device node or network interfaces.
	// All other devices return 1 here.
	IsInitialized() bool

	// Devlinks returns an Iterator over the device links pointing to the device file of the udev device.
	Devlinks() iter.Seq[string]

	// Properties returns an Iterator over the key/value device properties of the udev device.
	Properties() iter.Seq2[string, string]

	// Tags returns an Iterator over the tags attached to the udev device.
	Tags() iter.Seq[string]

	// Sysattrs returns an Iterator over the systems attributes of the udev device.
	Sysattrs() iter.Seq[string]

	// PropertyValue retrieves the value of a device property
	PropertyValue(key string) string

	// Driver returns the driver for the receiver
	Driver() string

	// Devnum returns the device major/minor number.
	Devnum() Devnum

	// Action returns the action for the event.
	// This is only valid if the device was received through a monitor.
	// Devices read from sys do not have an action string.
	// Usual actions are: add, remove, change, online, offline.
	Action() string

	// Seqnum returns the sequence number of the event.
	// This is only valid if the device was received through a monitor.
	// Devices read from sys do not have a sequence number.
	Seqnum() uint64

	// UsecSinceInitialized returns the number of microseconds passed since udev set up the device for the first time.
	// This is only implemented for devices with need to store properties in the udev database.
	// All other devices return 0 here.
	UsecSinceInitialized() uint64

	// SysattrValue retrieves the content of a sys attribute file, and returns an empty string if there is no sys attribute value.
	// The retrieved value is cached in the device.
	// Repeated calls will return the same value and not open the attribute again.
	SysattrValue(sysattr string) string

	// SetSysattrValue sets the content of a sys attribute file, and returns an error if this fails.
	SetSysattrValue(sysattr, value string) (err error)

	// HasTag checks if the udev device has the tag specified
	HasTag(tag string) bool
}

type DeviceEnumerator interface {

	// AddMatchSubsystem adds a filter for a subsystem of the device to include in the list.
	AddMatchSubsystem(subsystem string) (err error)

	// AddNomatchSubsystem adds a filter for a subsystem of the device to exclude from the list.
	AddNomatchSubsystem(subsystem string) (err error)

	// AddMatchSysattr adds a filter for a sys attribute at the device to include in the list.
	AddMatchSysattr(sysattr, value string) (err error)

	// AddNomatchSysattr adds a filter for a sys attribute at the device to exclude from the list.
	AddNomatchSysattr(sysattr, value string) (err error)

	// AddMatchProperty adds a filter for a property of the device to include in the list.
	AddMatchProperty(property, value string) (err error)

	// AddMatchSysname adds a filter for the name of the device to include in the list.
	AddMatchSysname(sysname string) (err error)

	// AddMatchTag adds a filter for a tag of the device to include in the list.
	AddMatchTag(tag string) (err error)

	// AddMatchParent adds a filter for a parent Device to include in the list.
	AddMatchParent(parent DeviceInfo) error

	// AddMatchIsInitialized adds a filter matching only devices which udev has set up already.
	// This makes sure, that the device node permissions and context are properly set and that network devices are fully renamed.
	// Usually, devices which are found in the kernel but not already handled by udev, have still pending events.
	// Services should subscribe to monitor events and wait for these devices to become ready, instead of using uninitialized devices.
	// For now, this will not affect devices which do not have a device node and are not network interfaces.
	AddMatchIsInitialized() (err error)

	// AddSyspath adds a device to the list of enumerated devices, to retrieve it back sorted in dependency order.
	AddSyspath(syspath string) (err error)

	// Devices returns an Iterator over the device syspaths matching the filter, sorted in dependency order.
	// The Iterator is using the github.com/jkeiser/iter package.
	// Values are returned as an interface{} and should be cast to string.
	Devices() (it iter.Seq[DeviceInfo], err error)

	// Subsystems returns an Iterator over the subsystem syspaths matching the filter, sorted in dependency order.
	// The Iterator is using the github.com/jkeiser/iter package.
	// Values are returned as an interface{} and should be cast to string.
	Subsystems() (it iter.Seq[string], err error)
}

type DeviceMonitor interface {
	// FD receives a file descriptor which can be checked for rediness
	FD() int

	EnableReceiving() (err error)

	ReceiveDevice() DeviceInfo

	// SetReceiveBufferSize sets the size of the kernel socket buffer.
	// This call needs the appropriate privileges to succeed.
	SetReceiveBufferSize(size int) (err error)

	// FilterAddMatchSubsystem adds a filter matching the device against a subsystem.
	// This filter is efficiently executed inside the kernel, and libudev subscribers will usually not be woken up for devices which do not match.
	// The filter must be installed before the monitor is switched to listening mode with the DeviceChan function.
	FilterAddMatchSubsystem(subsystem string) (err error)

	// FilterAddMatchSubsystemDevtype adds a filter matching the device against a subsystem and device type.
	// This filter is efficiently executed inside the kernel, and libudev subscribers will usually not be woken up for devices which do not match.
	// The filter must be installed before the monitor is switched to listening mode with the DeviceChan function.
	FilterAddMatchSubsystemDevtype(subsystem, devtype string) (err error)

	// FilterAddMatchTag adds a filter matching the device against a tag.
	// This filter is efficiently executed inside the kernel, and libudev subscribers will usually not be woken up for devices which do not match.
	// The filter must be installed before the monitor is switched to listening mode.
	FilterAddMatchTag(tag string) (err error)

	// FilterUpdate updates the installed socket filter.
	// This is only needed, if the filter was removed or changed.
	FilterUpdate() (err error)

	// FilterRemove removes all filter from the Monitor.
	FilterRemove() (err error)
}
