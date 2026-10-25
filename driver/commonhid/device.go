package commonhid

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/friedelschoen/go-wiimote"
	"github.com/friedelschoen/go-wiimote/internal/common"
)

type commonEvent struct {
	iface     wiimote.Feature
	timestamp time.Time
}

func (e commonEvent) Feature() wiimote.Feature { return e.iface }
func (e commonEvent) Timestamp() time.Time     { return e.timestamp }

type memResponse struct {
	err  error
	addr uint16
	data []byte
}

type device struct {
	wiimote.Poller[wiimote.Event]

	transport Transport

	openIfs map[wiimote.FeatureKind]feature
	// availIfs wiimote.FeatureKind

	// last known battery state
	battery uint8

	extensionKey [16]byte

	// wether an extension is currently connected
	hasExtension  bool
	extensionKind extensionKind

	// led state
	led    wiimote.Led
	rumble bool

	memory chan memResponse

	ackErr map[uint8]error
	ackMu  sync.Mutex

	// queued events (because a single report can generate multiple key events)
	events chan wiimote.Event
}

type memory struct {
	dev *device
}

func (m memory) Close() error {
	return nil
}

func (m memory) WriteAt(p []byte, off int64) (n int, err error) {
	return m.dev.writeMemory(memEEPROM, uint32(off), p)
}

func (m memory) ReadAt(p []byte, off int64) (n int, err error) {
	return m.dev.readMemory(memEEPROM, uint32(off), p)
}

type Transport interface {
	io.ReadWriteCloser
	FD() int
}

func NewDevice(transport Transport) wiimote.Device {
	d := &device{
		transport: transport,
		openIfs:   make(map[wiimote.FeatureKind]feature),
		events:    make(chan wiimote.Event, 64),
		memory:    make(chan memResponse, 16),
		ackErr:    make(map[uint8]error),
	}
	d.Poller = common.NewPoller(d)
	rand.Read(d.extensionKey[:])
	return d
}

// Feature interface
func (d *device) Kind() wiimote.FeatureKind { return wiimote.FeatureCore }
func (d *device) Device() wiimote.Device    { return d }
func (d *device) Opened() bool              { return true }
func (d *device) Close() error              { return nil }

func (d *device) Rumble(state bool) error {
	d.rumble = state
	return d.output(true, 0x10, 0x00)
}

func (d *device) Memory() (wiimote.Memory, error) {
	return memory{dev: d}, nil
}

func (d *device) Syspath() string { return "" }

func (d *device) String() string {
	return "wiimote-device (commonhid)"
}

func (d *device) updateReportMode() error {
	if len(d.openIfs) == 0 {
		// do nothing
		return nil
	}

	mode := findMode(maps.Values(d.openIfs))
	if mode == 0 {
		return fmt.Errorf("no mode found")
	}
	d.output(true, 0x20, 0x00, mode)
	return nil
}

func (d *device) OpenFeatures(ifaces wiimote.FeatureKind, wr bool) error {
	_ = wr // all features are writeable

	for k := wiimote.FeatureCore; k <= wiimote.FeatureGuitar; k <<= 1 {
		if _, ok := d.openIfs[k]; ifaces&k != 0 && !ok {
			d.openIfs[k] = d.featureFromName(k)
		}
	}
	return d.updateReportMode()
}

func (d *device) FD() int {
	return d.transport.FD()
}

func (d *device) Feature(kind wiimote.FeatureKind) wiimote.Feature {
	f, ok := d.openIfs[kind]
	if !ok {
		return nil
	}
	return f
}

func (d *device) Available(iface wiimote.FeatureKind) bool {
	if iface&wiimote.FeatureSetCore != 0 {
		return true
	}
	// TODO: if extension
	return false
}

func (d *device) LED() (wiimote.Led, error) {
	return d.led, nil
}

func (d *device) SetLED(leds wiimote.Led) error {
	return d.output(true, 0x11, byte(leds)<<4)
}

func (d *device) Battery() (uint, error) {
	return uint(d.battery), nil
}

func (d *device) DevType() (string, error) {
	return "unknown", os.ErrInvalid
}

func (d *device) Extension() (string, error) {
	return "none", os.ErrInvalid
}

func (d *device) Poll() (wiimote.Event, bool, error) {
	select {
	case ev := <-d.events:
		return ev, len(d.events) > 0, nil
	default:
	}

	d.readEvent()

	select {
	case ev := <-d.events:
		return ev, len(d.events) > 0, nil
	default:
		return nil, false, common.ErrWouldBlock
	}
}

func (d *device) decryptReport(report []byte) {
	currentMode := reportingModes[report[0]]
	address := 0x08
	for i, chr := range currentMode {
		if i >= len(report)-1 {
			// short report
			break
		}
		if chr != 'e' {
			continue
		}
		// if extension byte, decrypt
		report[i+1] = (report[i+1] ^ d.extensionKey[address%8]) + d.extensionKey[(address%8)+8]
		address++
	}
}

func (d *device) readEvent() {
	var buf [64]byte
	n, err := d.transport.Read(buf[:])
	if err != nil {
		// Non-blocking transport support (os.File O_NONBLOCK typically returns EAGAIN)
		if errors.Is(err, syscall.EAGAIN) {
			return
		}

		// Device gone
		if errors.Is(err, io.EOF) {
			d.events <- &wiimote.EventGone{
				Event: commonEvent{iface: nil, timestamp: time.Now()},
			}
			return
		}

		// Any other error: treat like "watch"/state changed; caller should reopen etc.
		d.events <- &wiimote.EventWatch{
			Event: commonEvent{iface: nil, timestamp: time.Now()},
		}
		return
	}
	if n <= 0 {
		return
	}

	report := buf[:n]
	ts := time.Now()

	rid := report[0]
	switch rid {
	case 0x20:
		d.battery = report[6]
		if report[3]&0x01 != 0 {
			d.battery = 0
		}
		extension := report[3]&0x02 != 0
		// throw event ?
		if extension != d.hasExtension {
			if extension {
				d.initExtension()
			}
			d.hasExtension = extension
		}
		d.led = wiimote.Led(report[3] >> 4)
		return

	case 0x21:
		size := (report[3] >> 4) + 1
		addr := uint16(report[4])<<8 | uint16(report[5])
		var err error
		switch report[3] & 0x0f {
		case 0:
			err = nil
		case 7:
			err = os.ErrPermission
		case 8:
			err = io.EOF
		default:
			err = os.ErrInvalid
		}
		d.memory <- memResponse{
			err:  err,
			addr: addr,
			data: report[6 : 6+size],
		}
		return

	case 0x22:
		ack := report[3]
		var err error
		if report[4] != 0 {
			err = os.ErrInvalid
		}
		d.ackMu.Lock()
		d.ackErr[ack] = err
		d.ackMu.Unlock()
		return
	}

	d.decryptReport(report)

	for _, f := range d.openIfs {
		f.acceptEvent(ts, report)
	}

	// TODO: extensions
}

func (d *device) initExtension() {
	//  0x55 to 0x(4)A400F0
	d.writeMemory(memREG, 0xa400f0, []byte{0x55})
	d.writeMemory(memREG, 0xa400fb, []byte{0x00})

	var kindb [6]byte
	d.readMemory(memREG, 0xa400fa, kindb[:])
	d.extensionKind = extensionKind(uint(kindb[0])<<16 | uint(kindb[2])<<8 | uint(kindb[2]))

	// After the identification is read, the encryption can be set up if required.
	// This is done by enabling encryption by writing 0xAA to extension register 0xF0,
	// and then writing the 16-byte key to register 0x40.
	// The key is written in 3 blocks of 6, 6, and 4 bytes.
	// (Writing a single encryption byte to register 0x40 will work on genuine Nintendo controllers, but not on others).
	// After this the extension can be operated in full encryption mode.

	// enable encryption
	d.writeMemory(memREG, 0xa400f0, []byte{0xaa})

	// write extension key
	d.writeMemory(memREG, 0xa40040, d.extensionKey[0:6])
	d.writeMemory(memREG, 0xa40046, d.extensionKey[6:12])
	d.writeMemory(memREG, 0xa4004c, d.extensionKey[12:16])
}

func (d *device) output(ack bool, report ...byte) error {
	if len(report) < 2 {
		panic("invalid report")
	}

	rid := report[0]

	if ack {
		// set 0x02 to enable acknowlegding
		report[1] |= 0x02
	}
	if d.rumble {
		report[1] |= 0x01
	}

	if ack {
		d.ackMu.Lock()
		delete(d.ackErr, rid)
		d.ackMu.Unlock()
	}

	if _, err := d.transport.Write(report); err != nil {
		return err
	}

	if !ack {
		return nil
	}

	for {
		d.ackMu.Lock()
		if err, ok := d.ackErr[rid]; ok {
			delete(d.ackErr, rid)
			d.ackMu.Unlock()
			return err
		}
		d.ackMu.Unlock()

		d.readEvent()
	}
}

func (d *device) enable(ack bool, report byte, enable bool) error {
	var enbyte byte
	if enable {
		enbyte = 0x04
	}
	return d.output(ack, report, enbyte)
}

/* memory spaces */
type memSpace uint8

const (
	memEEPROM memSpace = 0
	memREG    memSpace = 1
)

/*
readMemory reads from EEPROM or register space.
addr is 16-bit for EEPROM and 24-bit for regs (you pass it as uint32 either way).
Returns exactly len(dst) bytes on success.
*/
func (d *device) readMemory(space memSpace, addr uint32, dst []byte) (int, error) {
	if len(dst) == 0 {
		return 0, nil
	}

	off := 0
	for off < len(dst) {
		/* conservative chunk:
		   Many stacks use up to 16 bytes per reply, sometimes 32.
		   We'll request 16 to be safe across transports. */
		n := min(len(dst)-off, 16)

		/* send read request: 0x17 */
		ah, am, al := encodeAddr(space, addr+uint32(off))
		sizeHi, sizeLo := byte(n>>8), byte(n)
		if err := d.output(false, 0x17, 0x00, ah, am, al, sizeHi, sizeLo); err != nil {
			return off, err
		}

		var resp *memResponse
		for resp == nil {
			d.readEvent()
			select {
			case r := <-d.memory:
				resp = &r
			default:
			}
		}
		if resp.err != nil {
			return off, resp.err
		}
		gotAddr := resp.addr
		data := resp.data

		/* validate it’s the reply we asked for */
		if gotAddr != uint16((addr+uint32(off))&0xffff) {
			/* not fatal per se (could be another outstanding read),
			   but in this design we do synchronous single-flight reads,
			   so treat as invalid */
			return off, os.ErrInvalid
		}
		if len(data) < n {
			return off, io.ErrUnexpectedEOF
		}

		copy(dst[off:off+n], data[:n])
		off += n
	}

	return off, nil
}

/*
writeMemory writes to EEPROM or register space.
It chunks writes (max 16 bytes per write report payload here).
For EEPROM writes you often need delays
*/
func (d *device) writeMemory(space memSpace, addr uint32, src []byte) (int, error) {
	if len(src) == 0 {
		return 0, nil
	}

	off := 0
	for off < len(src) {
		n := min(len(src)-off, 16)

		ah, am, al := encodeAddr(space, addr+uint32(off))

		/* output report 0x16: [16][flags][addr_hi][addr_mid][addr_lo][size][data...] */
		rep := make([]byte, 0, 6+n)
		rep = append(rep, 0x16, 0x00, ah, am, al, byte(n))
		rep = append(rep, src[off:off+n]...)

		if err := d.output(false, rep...); err != nil {
			return off, err
		}

		/* EEPROM writes are slow; typical practice is small delay.
		   For registers it’s usually fine but harmless. */
		time.Sleep(10 * time.Millisecond)

		off += n
	}

	return off, nil
}

/* helper: encode address with space bit */
func encodeAddr(space memSpace, addr uint32) (hi, mid, lo byte) {
	/*
	   Wiimote convention: highest bit of hi-byte selects space:
	   - EEPROM: 0
	   - REG:    1
	   And remaining 23 bits are address.
	*/
	a := addr & 0x00ffffff
	hi = byte((a >> 16) & 0xff)
	if space == memREG {
		hi |= 0x10 /* NOTE: some docs say bit4 selects space; others use bit7.
		   If your reads fail, flip this to 0x80.
		   (I'm choosing 0x10 because that’s common in Wiibrew snippets.) */
	}
	mid = byte((a >> 8) & 0xff)
	lo = byte(a & 0xff)
	return
}
