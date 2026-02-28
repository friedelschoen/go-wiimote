package linuxkernel

// #include "input-defs.h"
import "C"
import (
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"github.com/friedelschoen/go-wiimote"
	"github.com/friedelschoen/go-wiimote/internal/common"
)

type Feature interface {
	wiimote.Feature

	fd() common.UnbufferedFile
	open(dev *Device, kind wiimote.FeatureKind, node string, wr bool) error
	acceptEvent(ts time.Time, event, code uint16, value int32) (wiimote.Event, error)
}

type commonFeature struct {
	// parent commoniface.device
	dev *Device
	// wether file is opened
	opened bool
	// Open file
	file common.UnbufferedFile
	// current kind
	kind wiimote.FeatureKind
}

func (iface *commonFeature) Kind() wiimote.FeatureKind {
	return iface.kind
}

func (iface *commonFeature) Device() wiimote.Device {
	return iface.dev
}
func (iface *commonFeature) fd() common.UnbufferedFile {
	return iface.file
}

// Opened returns a bitmask of opened features. Features may be closed due to
// error-conditions at any time. However, features are never opened
// automatically.
//
// You will get notified whenever this bitmask changes, except on explicit
// calls to Open() and Close(). See the wiimote.EventWatch event for more information.
func (iface *commonFeature) Opened() bool {
	return iface.opened
}

func (iff *commonFeature) open(dev *Device, kind wiimote.FeatureKind, node string, wr bool) error {
	if iff.dev != nil && iff.opened {
		return nil
	}

	iff.dev = dev

	flags := syscall.O_NONBLOCK | syscall.O_CLOEXEC
	if wr {
		flags |= syscall.O_RDWR
	}
	fd, err := syscall.Open(node, flags, 0)
	if err != nil {
		return err
	}
	file := common.UnbufferedFile(fd)

	var ep syscall.EpollEvent
	ep.Events = syscall.EPOLLIN
	ep.Fd = int32(fd)
	if err := syscall.EpollCtl(iff.dev.efd, syscall.EPOLL_CTL_ADD, int(fd), &ep); err != nil {
		file.Close()
		return err
	}

	iff.opened = true
	iff.file = file
	return nil
}

func (iff *commonFeature) Close() error {
	if !iff.opened {
		return nil
	}
	if err := syscall.EpollCtl(iff.dev.efd, syscall.EPOLL_CTL_DEL, int(iff.file), nil); err != nil {
		return err
	}
	if err := iff.file.Close(); err != nil {
		return err
	}
	iff.opened = false
	iff.file = 0

	delete(iff.dev.openIfs, iff.kind)
	return iff.dev.readNodes()
}

type rumbleFeature struct {
	commonFeature

	//  rumble-id for base-core feature force-feedback or -1
	rumbleValid bool
	rumbleID    int
}

func (iface *rumbleFeature) open(dev *Device, kind wiimote.FeatureKind, node string, wr bool) error {
	if err := iface.commonFeature.open(dev, kind, node, wr); err != nil {
		return err
	}

	return iface.uploadRumble()
}

// Upload the generic rumble event to the device. This may later be used for
// force-feedback effects. The event id is safed for later use.
func (iface *rumbleFeature) uploadRumble() error {
	effect := C.struct_ff_effect{
		_type: C.FF_RUMBLE,
		id:    -1,
	}

	rmb := (*C.struct_ff_rumble_effect)(unsafe.Pointer(&effect.u))
	rmb.strong_magnitude = 1

	if err := iface.file.Ioctl(C.EVIOCSFF, uintptr(unsafe.Pointer(&effect))); err != nil {
		return err
	}
	iface.rumbleValid = true
	iface.rumbleID = int(effect.id)
	return nil
}

func (iff *rumbleFeature) Close() error {
	iff.rumbleValid = false

	return iff.commonFeature.Close()
}

// Rumble sets the rumble motor.
//
// This requires the core-feature to be opened in writable mode.
func (dev *rumbleFeature) Rumble(state bool) error {
	if !dev.opened || !dev.rumbleValid {
		return os.ErrInvalid
	}

	var ev C.struct_input_event
	ev._type = C.EV_FF
	ev.code = C.ushort(dev.rumbleID)
	if state {
		ev.value = 1
	}

	buf := unsafe.Slice((*byte)(unsafe.Pointer(&ev)), unsafe.Sizeof(ev))

	n, err := dev.file.Write(buf)
	if err != nil {
		return err
	}
	if n != int(unsafe.Sizeof(ev)) {
		return io.ErrShortWrite
	}
	return nil
}

type FeatureCore struct {
	rumbleFeature
}

func (iface *FeatureCore) acceptEvent(ts time.Time, event, code uint16, value int32) (wiimote.Event, error) {
	if event != C.EV_KEY {
		return nil, nil
	}

	if value < 0 || value > 2 {
		return nil, nil
	}

	var key wiimote.Key
	switch code {
	case C.KEY_LEFT:
		key = wiimote.KeyLeft
	case C.KEY_RIGHT:
		key = wiimote.KeyRight
	case C.KEY_UP:
		key = wiimote.KeyUp
	case C.KEY_DOWN:
		key = wiimote.KeyDown
	case C.KEY_NEXT:
		key = wiimote.KeyPlus
	case C.KEY_PREVIOUS:
		key = wiimote.KeyMinus
	case C.BTN_1:
		key = wiimote.KeyOne
	case C.BTN_2:
		key = wiimote.KeyTwo
	case C.BTN_A:
		key = wiimote.KeyA
	case C.BTN_B:
		key = wiimote.KeyB
	case C.BTN_MODE:
		key = wiimote.KeyHome
	default:
		return nil, nil
	}

	var ev wiimote.EventKey
	ev.Event = commonEvent{iface, ts}
	ev.Code = key
	ev.State = wiimote.KeyState(value)
	return &ev, nil
}

// Memory
func (iface *FeatureCore) Memory() (wiimote.Memory, error) {
	id := filepath.Base(iface.dev.dev.Syspath())
	path := filepath.Join(debugfs, "hid", id, "eeprom")
	fd, err := syscall.Open(path, syscall.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	return &Memory{common.UnbufferedFile(fd)}, nil
}

type FeatureAccel struct {
	commonFeature

	accel wiimote.Vec3
}

func (iface *FeatureAccel) acceptEvent(ts time.Time, event, code uint16, value int32) (wiimote.Event, error) {
	if event == C.EV_SYN {
		var ev wiimote.EventAccel
		ev.Event = commonEvent{iface, ts}
		ev.Accel = iface.accel
		return &ev, nil
	}

	if event != C.EV_ABS {
		return nil, nil
	}

	switch code {
	case C.ABS_RX:
		iface.accel.X = value
	case C.ABS_RY:
		iface.accel.Y = value
	case C.ABS_RZ:
		iface.accel.Z = value
	}
	return nil, nil
}

type FeatureIR struct {
	commonFeature

	slots [4]wiimote.IRSlot
}

func (iface *FeatureIR) open(dev *Device, kind wiimote.FeatureKind, node string, wr bool) error {
	if err := iface.commonFeature.open(dev, kind, node, wr); err != nil {
		return err
	}
	for i := range iface.slots {
		iface.slots[i].X = 1023
		iface.slots[i].Y = 1023
	}
	return nil
}

func (iface *FeatureIR) acceptEvent(ts time.Time, event, code uint16, value int32) (wiimote.Event, error) {
	if event == C.EV_SYN {
		var ev wiimote.EventIR
		ev.Event = commonEvent{iface, ts}
		ev.Slots = iface.slots
		return &ev, nil
	}

	if event != C.EV_ABS {
		return nil, nil
	}

	switch code {
	case C.ABS_HAT0X:
		iface.slots[0].X = value
	case C.ABS_HAT0Y:
		iface.slots[0].Y = value
	case C.ABS_HAT1X:
		iface.slots[1].X = value
	case C.ABS_HAT1Y:
		iface.slots[1].Y = value
	case C.ABS_HAT2X:
		iface.slots[2].X = value
	case C.ABS_HAT2Y:
		iface.slots[2].Y = value
	case C.ABS_HAT3X:
		iface.slots[3].X = value
	case C.ABS_HAT3Y:
		iface.slots[3].Y = value
	}
	return nil, nil
}

type FeatureMotionPlus struct {
	commonFeature

	//  motion plus normalization
	normalizer     wiimote.Vec3 // event_abs
	normaizeFactor int32

	speed wiimote.Vec3
}

// SetMPNormalization sets Motion-Plus normalization and calibration values. The Motion-Plus sensor is very
// sensitive and may return really crappy values. This features allows to
// apply 3 absolute offsets x, y and z which are subtracted from any MP data
// before it is returned to the application. That is, if you set these values
// to 0, this has no effect (which is also the initial state).
//
// The calibration factor is used to perform runtime calibration. If
// it is 0 (the initial state), no runtime calibration is performed. Otherwise,
// the factor is used to re-calibrate the zero-point of MP data depending on MP
// input. This is an angoing calibration which modifies the internal state of
// the x, y and z values.
func (iface *FeatureMotionPlus) SetMPNormalization(x, y, z, factor int32) {
	iface.normalizer.X = x * 100
	iface.normalizer.Y = y * 100
	iface.normalizer.Z = z * 100
	iface.normaizeFactor = factor
}

// MPNormalization reads the Motion-Plus normalization and calibration values. Please see
// SetMPNormalization() how this is handled.
//
// Note that if the calibration factor is not 0, the normalization values may
// change depending on incoming MP data. Therefore, the data read via this
// function may differ from the values that you wrote to previously. However,
// apart from applied calibration, these value are the same as were set
// previously via SetMPNormalization() and you can feed them back
// in later.
func (iface *FeatureMotionPlus) MPNormalization() (x, y, z, factor int32) {
	return iface.normalizer.X / 100,
		iface.normalizer.Y / 100,
		iface.normalizer.Z / 100,
		iface.normaizeFactor
}

func (iface *FeatureMotionPlus) acceptEvent(ts time.Time, event, code uint16, value int32) (wiimote.Event, error) {
	if event == C.EV_SYN {
		iface.speed.X -= iface.normalizer.X / 100
		iface.speed.Y -= iface.normalizer.Y / 100
		iface.speed.Z -= iface.normalizer.Z / 100
		if iface.speed.X > 0 {
			iface.normalizer.X += iface.normaizeFactor
		} else {
			iface.normalizer.X -= iface.normaizeFactor
		}
		if iface.speed.Y > 0 {
			iface.normalizer.Y += iface.normaizeFactor
		} else {
			iface.normalizer.Y -= iface.normaizeFactor
		}
		if iface.speed.Z > 0 {
			iface.normalizer.Z += iface.normaizeFactor
		} else {
			iface.normalizer.Z -= iface.normaizeFactor
		}

		var ev wiimote.EventMotionPlus
		ev.Event = commonEvent{iface, ts}
		ev.Speed = iface.speed
		return &ev, nil
	}

	if event != C.EV_ABS {
		return nil, nil
	}

	switch code {
	case C.ABS_RX:
		iface.speed.X = value
	case C.ABS_RY:
		iface.speed.Y = value
	case C.ABS_RZ:
		iface.speed.Z = value
	}

	return nil, nil
}

type FeatureNunchuck struct {
	commonFeature

	stick wiimote.Vec2
	accel wiimote.Vec3
}

func (iface *FeatureNunchuck) acceptEvent(ts time.Time, event, code uint16, value int32) (wiimote.Event, error) {
	switch event {
	case C.EV_KEY:
		if value < 0 || value > 2 {
			return nil, nil
		}
		var key wiimote.Key
		switch code {
		case C.BTN_C:
			key = wiimote.KeyC
		case C.BTN_Z:
			key = wiimote.KeyZ
		default:
			return nil, nil
		}

		var ev wiimote.EventNunchukKey
		ev.Event = commonEvent{iface, ts}
		ev.Code = key
		ev.State = wiimote.KeyState(value)
		return &ev, nil
	case C.EV_ABS:
		switch code {
		case C.ABS_HAT0X:
			iface.stick.X = value
		case C.ABS_HAT0Y:
			iface.stick.Y = value
		case C.ABS_RX:
			iface.accel.X = value
		case C.ABS_RY:
			iface.accel.Y = value
		case C.ABS_RZ:
			iface.accel.Z = value
		}
	case C.EV_SYN:
		var ev wiimote.EventNunchukMove
		ev.Event = commonEvent{iface, ts}
		ev.Stick = iface.stick
		ev.Accel = iface.accel
		return &ev, nil
	}

	return nil, nil
}

type FeatureClassicController struct {
	commonFeature

	stickLeft     wiimote.Vec2
	stickRight    wiimote.Vec2
	shoulderLeft  int32
	shoulderRight int32
}

func (iface *FeatureClassicController) acceptEvent(ts time.Time, event, code uint16, value int32) (wiimote.Event, error) {
	switch event {
	case C.EV_KEY:
		if value < 0 || value > 2 {
			return nil, nil
		}

		var key wiimote.Key
		switch code {
		case C.BTN_A:
			key = wiimote.KeyA
		case C.BTN_B:
			key = wiimote.KeyB
		case C.BTN_X:
			key = wiimote.KeyX
		case C.BTN_Y:
			key = wiimote.KeyY
		case C.KEY_NEXT:
			key = wiimote.KeyPlus
		case C.KEY_PREVIOUS:
			key = wiimote.KeyMinus
		case C.BTN_MODE:
			key = wiimote.KeyHome
		case C.KEY_LEFT:
			key = wiimote.KeyLeft
		case C.KEY_RIGHT:
			key = wiimote.KeyRight
		case C.KEY_UP:
			key = wiimote.KeyUp
		case C.KEY_DOWN:
			key = wiimote.KeyDown
		case C.BTN_TL:
			key = wiimote.KeyTL
		case C.BTN_TR:
			key = wiimote.KeyTR
		case C.BTN_TL2:
			key = wiimote.KeyZL
		case C.BTN_TR2:
			key = wiimote.KeyZR
		default:
			return nil, nil
		}

		var ev wiimote.EventClassicControllerKey
		ev.Event = commonEvent{iface, ts}
		ev.Code = key
		ev.State = wiimote.KeyState(value)
		return &ev, nil
	case C.EV_ABS:
		switch code {
		case C.ABS_HAT1X:
			iface.stickLeft.X = value
		case C.ABS_HAT1Y:
			iface.stickLeft.Y = value
		case C.ABS_HAT2X:
			iface.stickRight.X = value
		case C.ABS_HAT2Y:
			iface.stickRight.Y = value
		case C.ABS_HAT3X:
			iface.shoulderLeft = value
		case C.ABS_HAT3Y:
			iface.shoulderRight = value
		}
	case C.EV_SYN:
		var ev wiimote.EventClassicControllerMove
		ev.Event = commonEvent{iface, ts}
		ev.StickLeft = iface.stickLeft
		ev.StickRight = iface.stickRight
		ev.ShoulderLeft = iface.shoulderLeft
		ev.ShoulderRight = iface.shoulderRight
		return &ev, nil
	}

	return nil, nil
}

type FeatureBalanceBoard struct {
	commonFeature

	weights [4]int32
}

func (iface *FeatureBalanceBoard) acceptEvent(ts time.Time, event, code uint16, value int32) (wiimote.Event, error) {
	if event == C.EV_SYN {
		var ev wiimote.EventBalanceBoard
		ev.Event = commonEvent{iface, ts}
		ev.Weights = iface.weights
		return &ev, nil
	}

	if event != C.EV_ABS {
		return nil, nil
	}

	switch code {
	case C.ABS_HAT0X:
		iface.weights[0] = value
	case C.ABS_HAT0Y:
		iface.weights[1] = value
	case C.ABS_HAT1X:
		iface.weights[2] = value
	case C.ABS_HAT1Y:
		iface.weights[3] = value
	}

	return nil, nil
}

type FeatureProController struct {
	rumbleFeature

	sticks [2]wiimote.Vec2
}

func (iface *FeatureProController) acceptEvent(ts time.Time, event, code uint16, value int32) (wiimote.Event, error) {
	switch event {
	case C.EV_KEY:
		if value < 0 || value > 2 {
			return nil, nil
		}

		var key wiimote.Key
		switch code {
		case C.BTN_EAST:
			key = wiimote.KeyA
		case C.BTN_SOUTH:
			key = wiimote.KeyB
		case C.BTN_NORTH:
			key = wiimote.KeyX
		case C.BTN_WEST:
			key = wiimote.KeyY
		case C.BTN_START:
			key = wiimote.KeyPlus
		case C.BTN_SELECT:
			key = wiimote.KeyMinus
		case C.BTN_MODE:
			key = wiimote.KeyHome
		case C.BTN_DPAD_LEFT:
			key = wiimote.KeyLeft
		case C.BTN_DPAD_RIGHT:
			key = wiimote.KeyRight
		case C.BTN_DPAD_UP:
			key = wiimote.KeyUp
		case C.BTN_DPAD_DOWN:
			key = wiimote.KeyDown
		case C.BTN_TL:
			key = wiimote.KeyTL
		case C.BTN_TR:
			key = wiimote.KeyTR
		case C.BTN_TL2:
			key = wiimote.KeyZL
		case C.BTN_TR2:
			key = wiimote.KeyZR
		case C.BTN_THUMBL:
			key = wiimote.KeyThumbL
		case C.BTN_THUMBR:
			key = wiimote.KeyThumbR
		default:
			return nil, nil
		}

		var ev wiimote.EventProControllerKey
		ev.Event = commonEvent{iface, ts}
		ev.Code = key
		ev.State = wiimote.KeyState(value)
		return &ev, nil
	case C.EV_ABS:
		switch code {
		case C.ABS_X:
			iface.sticks[0].X = value
		case C.ABS_Y:
			iface.sticks[0].Y = value
		case C.ABS_RX:
			iface.sticks[1].X = value
		case C.ABS_RY:
			iface.sticks[1].Y = value
		}
	case C.EV_SYN:
		var ev wiimote.EventProControllerMove
		ev.Event = commonEvent{iface, ts}
		ev.Sticks = iface.sticks
		return &ev, nil
	}

	return nil, nil
}

type FeatureDrums struct {
	commonFeature

	pad         wiimote.Vec2
	cymbalLeft  int32
	cymbalRight int32
	tomLeft     int32
	tomRight    int32
	tomFarRight int32
	bass        int32
	hiHat       int32
}

func (iface *FeatureDrums) acceptEvent(ts time.Time, event, code uint16, value int32) (wiimote.Event, error) {
	switch event {
	case C.EV_KEY:
		if value < 0 || value > 2 {
			return nil, nil
		}

		var key wiimote.Key
		switch code {
		case C.BTN_START:
			key = wiimote.KeyPlus
		case C.BTN_SELECT:
			key = wiimote.KeyMinus
		default:
			return nil, nil
		}

		var ev wiimote.EventDrumsKey
		ev.Event = commonEvent{iface, ts}
		ev.Code = key
		ev.State = wiimote.KeyState(value)
		return &ev, nil
	case C.EV_ABS:
		switch code {
		case C.ABS_X:
			iface.pad.X = value
		case C.ABS_Y:
			iface.pad.Y = value
		case C.ABS_CYMBAL_LEFT:
			iface.cymbalLeft = value
		case C.ABS_CYMBAL_RIGHT:
			iface.cymbalRight = value
		case C.ABS_TOM_LEFT:
			iface.tomLeft = value
		case C.ABS_TOM_RIGHT:
			iface.tomRight = value
		case C.ABS_TOM_FAR_RIGHT:
			iface.tomFarRight = value
		case C.ABS_BASS:
			iface.bass = value
		case C.ABS_HI_HAT:
			iface.hiHat = value
		}
	case C.EV_SYN:
		var ev wiimote.EventDrumsMove
		ev.Event = commonEvent{iface, ts}
		ev.Pad = iface.pad
		ev.CymbalLeft = iface.cymbalLeft
		ev.CymbalRight = iface.cymbalRight
		ev.TomLeft = iface.tomLeft
		ev.TomRight = iface.tomRight
		ev.TomFarRight = iface.tomFarRight
		ev.Bass = iface.bass
		ev.HiHat = iface.hiHat
		return &ev, nil
	}

	return nil, nil
}

type FeatureGuitar struct {
	commonFeature

	stick     wiimote.Vec2
	whammyBar int32
	fretBar   int32
}

func (iface *FeatureGuitar) acceptEvent(ts time.Time, event, code uint16, value int32) (wiimote.Event, error) {
	switch event {
	case C.EV_KEY:
		if value < 0 || value > 2 {
			return nil, nil
		}

		var key wiimote.Key
		switch code {
		case C.BTN_FRET_FAR_UP:
			key = wiimote.KeyFretFarUp
		case C.BTN_FRET_UP:
			key = wiimote.KeyFretUp
		case C.BTN_FRET_MID:
			key = wiimote.KeyFretMid
		case C.BTN_FRET_LOW:
			key = wiimote.KeyFretLow
		case C.BTN_FRET_FAR_LOW:
			key = wiimote.KeyFretFarLow
		case C.BTN_STRUM_BAR_UP:
			key = wiimote.KeyStrumBarUp
		case C.BTN_STRUM_BAR_DOWN:
			key = wiimote.KeyStrumBarDown
		case C.BTN_START:
			key = wiimote.KeyPlus
		case C.BTN_MODE:
			key = wiimote.KeyHome
		default:
			return nil, nil
		}

		var ev wiimote.EventGuitarKey
		ev.Event = commonEvent{iface, ts}
		ev.Code = key
		ev.State = wiimote.KeyState(value)
		return &ev, nil
	case C.EV_ABS:
		switch code {
		case C.ABS_X:
			iface.stick.X = value
		case C.ABS_Y:
			iface.stick.Y = value
		case C.ABS_WHAMMY_BAR:
			iface.whammyBar = value
		case C.ABS_FRET_BOARD:
			iface.fretBar = value
		}
	case C.EV_SYN:
		var ev wiimote.EventGuitarMove
		ev.Event = commonEvent{iface, ts}
		ev.Stick = iface.stick
		ev.WhammyBar = iface.whammyBar
		ev.FretBar = iface.fretBar
		return &ev, nil
	}

	return nil, nil
}

func FeatureKindFromName(name string) (wiimote.FeatureKind, bool) {
	switch name {
	case "Nintendo Wii Remote":
		return wiimote.FeatureCore, true
	case "Nintendo Wii Remote Accelerometer":
		return wiimote.FeatureAccel, true
	case "Nintendo Wii Remote IR":
		return wiimote.FeatureIR, true
	case "Nintendo Wii Remote Motion Plus":
		return wiimote.FeatureMotionPlus, true
	case "Nintendo Wii Remote Nunchuk":
		return wiimote.FeatureNunchuck, true
	case "Nintendo Wii Remote Classic Controller":
		return wiimote.FeatureClassicController, true
	case "Nintendo Wii Remote Balance Board":
		return wiimote.FeatureBalanceBoard, true
	case "Nintendo Wii Remote Pro Controller":
		return wiimote.FeatureProController, true
	case "Nintendo Wii Remote Drums":
		return wiimote.FeatureDrums, true
	case "Nintendo Wii Remote Guitar":
		return wiimote.FeatureGuitar, true
	default:
		return 0, false
	}
}

func FeatureFromName(kind wiimote.FeatureKind) Feature {
	switch kind {
	case wiimote.FeatureCore:
		return &FeatureCore{}
	case wiimote.FeatureAccel:
		return &FeatureAccel{}
	case wiimote.FeatureIR:
		return &FeatureIR{}
	case wiimote.FeatureMotionPlus:
		return &FeatureMotionPlus{}
	case wiimote.FeatureNunchuck:
		return &FeatureNunchuck{}
	case wiimote.FeatureClassicController:
		return &FeatureClassicController{}
	case wiimote.FeatureBalanceBoard:
		return &FeatureBalanceBoard{}
	case wiimote.FeatureProController:
		return &FeatureProController{}
	case wiimote.FeatureDrums:
		return &FeatureDrums{}
	case wiimote.FeatureGuitar:
		return &FeatureGuitar{}
	}
	return nil
}

func readEvent(fd common.UnbufferedFile) (*C.struct_input_event, error) {
	var ev C.struct_input_event
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&ev)), unsafe.Sizeof(ev))

	n, err := fd.Read(buf)
	if err == syscall.EAGAIN {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if n != int(unsafe.Sizeof(ev)) {
		return nil, io.ErrShortBuffer
	}
	return &ev, nil
}

func dispatchEvent(iff Feature) (wiimote.Event, error) {
	for {
		input, err := readEvent(iff.fd())
		if err != nil {
			iff.Close()
			return &wiimote.EventWatch{
				Event: commonEvent{iff, time.Now()},
			}, nil
		}
		if input == nil {
			return nil, common.ErrPollAgain
		}
		ts := cTime(input.time)
		eventType := uint16(input._type)
		code := uint16(input.code)
		value := int32(input.value)

		event, err := iff.acceptEvent(ts, eventType, code, value)
		if event != nil || err != nil {
			return event, err
		}
	}
}
