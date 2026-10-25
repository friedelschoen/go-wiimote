package commonhid

import (
	"errors"
	"time"

	"github.com/friedelschoen/go-wiimote"
)

type extensionKind uint

const (
	extensionBladeFX                     extensionKind = 0x000000000000 //BladeFX adapter that has been initialised old way, unplugged then replugged
	extensionOther                       extensionKind = 0xFFFFFFFFFFFF // GHWT Guitar or Drums or synced BladeFX or other wireless nunchuk initialised the old way
	extensionNunchuck                    extensionKind = 0x0000A4200000 // Nunchuk
	extensionClassicController           extensionKind = 0x0000A4200101 // Classic Controller
	extensionClassicControllerPro        extensionKind = 0x0100A4200101 // Classic Controller Pro & NES/SNES controller from the NES/SNES mini
	extensionDrawsomeGraphicsTablet      extensionKind = 0xFF00A4200013 // Drawsome Graphics Tablet
	extensionGHGuitar                    extensionKind = 0x0000A4200103 // GH3 or GHWT Guitar
	extensionGHDrums                     extensionKind = 0x0100A4200103 // Guitar Hero World Tour Drums
	extensionDJHTurntable                extensionKind = 0x0300A4200103 // DJ Hero Turntable
	extensionTNTTaTaCon                  extensionKind = 0x0000A4200111 // Taiko no Tatsujin TaTaCon (Drum controller)
	extensionUDrawGameTablet             extensionKind = 0xFF00A4200112 // uDraw GameTablet (Graphics tablet)
	extensionShinkansenController        extensionKind = 0x0000A4200310 // Densha de GO! Shinkansen Controller
	extensionBalanceBoard                extensionKind = 0x0000A4200402 // Wii Balance Board
	extensionInactiveMotionPlus          extensionKind = 0x0000A4200005 // Inactive Wii Motion Plus (Built-in)
	extensionActiveMotionPlus            extensionKind = 0x0000A4200405 // Activated Wii Motion Plus
	extensionMotionPlusNunchuck          extensionKind = 0x0000A4200505 // Activated Wii Motion Plus in Nunchuck passthrough mode
	extensionMotionPlusClassicController extensionKind = 0x0000A4200705 // Activated Wii Motion Plus in Classic Controller passthrough mode
)

type modeRequirement struct {
	feature rune
	min     int
	max     int
}

type feature interface {
	wiimote.Feature

	enable(en bool) error
	acceptEvent(ts time.Time, report []byte)
	requireModes() []modeRequirement
}

type commonFeature struct {
	kind wiimote.FeatureKind
	dev  *device
}

func (f commonFeature) Kind() wiimote.FeatureKind {
	return f.kind
}

func (f commonFeature) Device() wiimote.Device {
	return f.dev
}

func (f commonFeature) Opened() bool {
	// return f.dev.openIfs&f.kind != 0
	return true
}

func (f commonFeature) Close() error {
	// f.dev.openIfs &^= f.kind
	return f.dev.updateReportMode()
}

func (f commonFeature) enable(en bool) error {
	return nil
}

func (f commonFeature) requireModes() []modeRequirement {
	return nil
}

type featureCore struct {
	commonFeature

	// last known button state (16-bit)
	btnPrev uint16
}

func (f *featureCore) requireModes() []modeRequirement {
	return []modeRequirement{
		{feature: 'b', min: 2, max: 3},
	}
}

func (f *featureCore) emitButtons(ts time.Time, btn uint16) {
	if btn == f.btnPrev {
		return
	}

	changed := btn ^ f.btnPrev
	f.btnPrev = btn

	emitChanged := func(k wiimote.Key, value uint16) {
		if changed&value == 0 {
			return
		}
		pressed := btn&value != 0
		f.dev.events <- &wiimote.EventKey{
			Event:   commonEvent{iface: f, timestamp: ts},
			Code:    k,
			Pressed: pressed,
		}
	}

	emitChanged(wiimote.KeyTwo, 0x0001)
	emitChanged(wiimote.KeyOne, 0x0002)
	emitChanged(wiimote.KeyB, 0x0004)
	emitChanged(wiimote.KeyA, 0x0008)
	emitChanged(wiimote.KeyMinus, 0x0010)
	emitChanged(wiimote.KeyHome, 0x0080)
	emitChanged(wiimote.KeyLeft, 0x0100)
	emitChanged(wiimote.KeyRight, 0x0200)
	emitChanged(wiimote.KeyDown, 0x0400)
	emitChanged(wiimote.KeyUp, 0x0800)
	emitChanged(wiimote.KeyPlus, 0x1000)
}

func (f *featureCore) acceptEvent(ts time.Time, report []byte) {
	rid := report[0]
	if rid == 0x20 || rid == 0x21 || rid == 0x22 || rid == 0x30 || rid == 0x31 || rid == 0x33 || rid == 0x35 || rid == 0x37 || rid == 0x3e || rid == 0x3f {
		btn := uint16(report[1])<<8 | uint16(report[2])
		f.emitButtons(ts, btn)
	}
}

type featureAccel struct {
	commonFeature

	interleaved [19]byte
}

func (f *featureAccel) requireModes() []modeRequirement {
	return []modeRequirement{
		{feature: 'a', min: 3, max: 3},
	}
}

func (d *featureAccel) acceptEvent(ts time.Time, report []byte) {
	rid := report[0]
	if rid == 0x31 || rid == 0x33 || rid == 0x35 || rid == 0x37 {
		var accel wiimote.Vec3
		accel.X = (int32(report[3])<<2 | int32(report[1]>>5)&0x03) - 0x80
		accel.Y = (int32(report[4])<<2 | int32(report[1]>>4)&0x03) - 0x80
		accel.Z = (int32(report[5])<<2 | int32(report[1]>>5)&0x02) - 0x80

		d.dev.events <- &wiimote.EventAccel{
			Event: commonEvent{iface: d, timestamp: ts},
			Accel: accel,
		}
	}
	if rid == 0x3e {
		copy(d.interleaved[:], report[4:])
	}
	if rid == 0x3f {
		var accel wiimote.Vec3
		accel.X = int32(d.interleaved[3])
		accel.Y = int32(report[3])
		accel.Z = (int32(d.interleaved[0]>>5)&0x03)<<6 |
			(int32(d.interleaved[1]>>5)&0x03)<<8 |
			(int32(report[0]>>5)&0x03)<<2 |
			(int32(report[1]>>5)&0x03)<<4

		d.dev.events <- &wiimote.EventAccel{
			Event: commonEvent{iface: d, timestamp: ts},
			Accel: accel,
		}
	}
}

type featureIR struct {
	commonFeature

	irfull      bool
	interleaved [4]wiimote.IRSlot
}

// Device interface
func (d *featureIR) IRFull() bool { return d.irfull }

func (d *featureIR) SetIRFull(fullreport bool) { d.irfull = fullreport }

func (f *featureIR) requireModes() []modeRequirement {
	if f.irfull {
		return []modeRequirement{
			{feature: 'i', min: 13, max: 64},
		}
	}

	return []modeRequirement{
		{feature: 'i', min: 13, max: 16}, // 0x33 extended preferred
		{feature: 'i', min: 10, max: 12}, // 0x36/0x37 basic fallback
	}
}

func (f *featureIR) enable(en bool) error {
	if err := f.commonFeature.enable(en); err != nil {
		return err
	}

	err1 := f.dev.enable(true, 0x13, en)
	err2 := f.dev.enable(true, 0x1a, en)
	return errors.Join(err1, err2)
}

func (f *featureIR) emitIRBasic(ts time.Time, report []byte) {
	var slots [4]wiimote.IRSlot
	slots[0].X = int32(report[0]) | (int32(report[2]>>4)&0x03)<<8
	slots[0].Y = int32(report[1]) | (int32(report[2]>>6)&0x03)<<8
	slots[1].X = int32(report[3]) | (int32(report[2]>>0)&0x03)<<8
	slots[1].Y = int32(report[4]) | (int32(report[2]>>2)&0x03)<<8

	slots[2].X = int32(report[5]) | (int32(report[7]>>4)&0x03)<<8
	slots[2].Y = int32(report[6]) | (int32(report[7]>>6)&0x03)<<8
	slots[3].X = int32(report[8]) | (int32(report[7]>>0)&0x03)<<8
	slots[3].Y = int32(report[9]) | (int32(report[7]>>2)&0x03)<<8

	f.dev.events <- &wiimote.EventIR{
		Event: commonEvent{iface: f, timestamp: ts},
		Slots: slots,
	}
}

func (f *featureIR) setSlotExtended(r []byte) (slot wiimote.IRSlot) {
	slot.X = int32(r[0]) | (int32(r[2]>>4)&0x03)<<8
	slot.Y = int32(r[1]) | (int32(r[2]>>6)&0x03)<<8
	slot.Size = r[2] & 0x0f
	return
}

func (f *featureIR) emitIRExtended(ts time.Time, report []byte) {
	var slots [4]wiimote.IRSlot
	slots[0] = f.setSlotExtended(report[0:])
	slots[1] = f.setSlotExtended(report[3:])
	slots[2] = f.setSlotExtended(report[6:])
	slots[3] = f.setSlotExtended(report[9:])

	f.dev.events <- &wiimote.EventIR{
		Event: commonEvent{iface: f, timestamp: ts},
		Slots: slots,
	}
}

func (f *featureIR) setSlotInterleaved(r []byte) (slot wiimote.IRSlot) {
	slot = f.setSlotExtended(r)
	slot.Bounds.Min.X = int32(r[3] & 0x7f)
	slot.Bounds.Min.Y = int32(r[4] & 0x7f)
	slot.Bounds.Max.X = int32(r[5] & 0x7f)
	slot.Bounds.Max.Y = int32(r[6] & 0x7f)
	slot.Intensity = r[8]
	return
}

func (f *featureIR) acceptEvent(ts time.Time, report []byte) {
	rid := report[0]
	if rid == 0x33 {
		// extended
		f.emitIRExtended(ts, report[6:18])
	}
	if rid == 0x36 {
		// basic
		f.emitIRBasic(ts, report[3:13])
	}
	if rid == 0x37 {
		// basic
		f.emitIRBasic(ts, report[6:16])
	}
	if rid == 0x3e {
		f.interleaved[0] = f.setSlotInterleaved(report[0:])
		f.interleaved[1] = f.setSlotInterleaved(report[9:])
	}
	if rid == 0x3f {
		f.interleaved[2] = f.setSlotInterleaved(report[0:])
		f.interleaved[3] = f.setSlotInterleaved(report[9:])

		f.dev.events <- &wiimote.EventIR{
			Event: commonEvent{iface: f, timestamp: ts},
			Slots: f.interleaved,
		}
	}
}

type featureMotionPlus struct {
	commonFeature

	//  motion plus normalization
	normalizer     wiimote.Vec3 // event_abs
	normaizeFactor int32

	speed wiimote.Vec3
}

func (f *featureMotionPlus) requireModes() []modeRequirement {
	return []modeRequirement{
		{feature: 'e', min: 6, max: 32},
	}
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
func (iface *featureMotionPlus) SetMPNormalization(x, y, z, factor int32) {
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
func (iface *featureMotionPlus) MPNormalization() (x, y, z, factor int32) {
	return iface.normalizer.X / 100,
		iface.normalizer.Y / 100,
		iface.normalizer.Z / 100,
		iface.normaizeFactor
}

func (iface *featureMotionPlus) acceptEvent(ts time.Time, report []byte) {
}

type featureNunchuck struct {
	commonFeature

	btnPrev uint8
}

func (f *featureNunchuck) requireModes() []modeRequirement {
	return []modeRequirement{
		{feature: 'e', min: 6, max: 32},
	}
}

func (f *featureNunchuck) emitButtons(ts time.Time, btn uint8) {
	if btn == f.btnPrev {
		return
	}

	changed := btn ^ f.btnPrev
	f.btnPrev = btn

	emitChanged := func(k wiimote.Key, value uint8) {
		if changed&value == 0 {
			return
		}
		pressed := btn&value != 0
		f.dev.events <- &wiimote.EventKey{
			Event:   commonEvent{iface: f, timestamp: ts},
			Code:    k,
			Pressed: pressed,
		}
	}

	emitChanged(wiimote.KeyZ, 0x0001)
	emitChanged(wiimote.KeyC, 0x0002)
}

func (f *featureNunchuck) emitEvent(ts time.Time, report []byte) {
	f.emitButtons(ts, report[5])

	var stick wiimote.Vec2
	stick.X = int32(report[0])
	stick.Y = int32(report[1])

	var accel wiimote.Vec3
	accel.X = int32(report[2])<<2 | int32(report[5])<<2
	accel.Y = int32(report[3])<<2 | int32(report[5])<<4
	accel.Z = int32(report[4])<<2 | int32(report[5])<<6

	f.dev.events <- &wiimote.EventNunchukMove{
		Event: commonEvent{iface: f, timestamp: ts},
		Stick: stick,
		Accel: accel,
	}
}

func (f *featureNunchuck) acceptEvent(ts time.Time, report []byte) {
	rid := report[0]
	if rid == 0x32 || rid == 0x34 {
		f.emitEvent(ts, report[3:9])
	}
	if rid == 0x35 {
		f.emitEvent(ts, report[6:12])
	}
	if rid == 0x36 {
		f.emitEvent(ts, report[13:19])
	}
	if rid == 0x37 {
		f.emitEvent(ts, report[16:22])
	}
	if rid == 0x3d {
		f.emitEvent(ts, report[1:7])
	}
}

type featureClassicController struct {
	commonFeature

	stickLeft     wiimote.Vec2
	stickRight    wiimote.Vec2
	shoulderLeft  int32
	shoulderRight int32
}

func (iface *featureClassicController) acceptEvent(ts time.Time, report []byte) {
}

type featureBalanceBoard struct {
	commonFeature

	weights [4]int32
}

func (iface *featureBalanceBoard) acceptEvent(ts time.Time, report []byte) {
}

type featureProController struct {
	commonFeature

	sticks [2]wiimote.Vec2
}

func (iface *featureProController) acceptEvent(ts time.Time, report []byte) {
}

type featureDrums struct {
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

func (iface *featureDrums) acceptEvent(ts time.Time, report []byte) {
}

type featureGuitar struct {
	commonFeature

	stick     wiimote.Vec2
	whammyBar int32
	fretBar   int32
}

func (iface *featureGuitar) acceptEvent(ts time.Time, report []byte) {
}

func (dev *device) featureFromName(kind wiimote.FeatureKind) feature {
	f := commonFeature{dev: dev, kind: kind}
	switch kind {
	case wiimote.FeatureCore:
		return &featureCore{commonFeature: f}
	case wiimote.FeatureAccel:
		return &featureAccel{commonFeature: f}
	case wiimote.FeatureIR:
		return &featureIR{commonFeature: f}
	case wiimote.FeatureMotionPlus:
		return &featureMotionPlus{commonFeature: f}
	case wiimote.FeatureNunchuck:
		return &featureNunchuck{commonFeature: f}
	case wiimote.FeatureClassicController:
		return &featureClassicController{commonFeature: f}
	case wiimote.FeatureBalanceBoard:
		return &featureBalanceBoard{commonFeature: f}
	case wiimote.FeatureProController:
		return &featureProController{commonFeature: f}
	case wiimote.FeatureDrums:
		return &featureDrums{commonFeature: f}
	case wiimote.FeatureGuitar:
		return &featureGuitar{commonFeature: f}
	}
	return nil
}
