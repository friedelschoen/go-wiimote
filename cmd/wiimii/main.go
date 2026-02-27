package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/friedelschoen/go-wiimote"
	"github.com/friedelschoen/go-wiimote/backend"
	"github.com/friedelschoen/go-wiimote/discovery"
	"github.com/friedelschoen/go-wiimote/pkg/eeprom"
)

func watchDevice(dev wiimote.Device) {
	fmt.Printf("new device: %s\n", dev.String())
	time.Sleep(100 * time.Millisecond)

	// coreif := wiimote.InterfaceCore{}
	if err := dev.OpenInterfaces(true, wiimote.InterfaceCore); err != nil {
		fmt.Fprintf(os.Stderr, "error: unable to open device: %s", err)
	}

	var mif wiimote.MemoryInterface
	for {
		ev, err := dev.Wait(-1)
		if err != nil && ev == nil {
			continue
		}
		if ifev, ok := ev.(*wiimote.EventInterface); ok {
			if i, ok := ifev.Interface().(wiimote.MemoryInterface); ok {
				mif = i
				break
			}
		}
	}

	f, err := mif.Memory()
	if err != nil {
		log.Fatalln(err)
	}
	defer f.Close()
	block, err := eeprom.ReadMiiBlock(f)
	if err != nil {
		log.Fatalln(err)
	}
	for slot := range block.MiiSlotSeq() {
		mii := eeprom.DecodeMii(slot)
		fmt.Printf("%q by %q\n", mii.Name, mii.CreatorName)
	}
}

func main() {
	flag.Parse()

	monitor, err := discovery.NewWiimoteMonitor()
	if err != nil {
		log.Fatalln("error: ", err)
	}

	fmt.Println("waiting for devices...")
	dev, err := monitor.Wait(-1)
	if err != nil || dev == nil {
		log.Printf("error while polling: %v\n", err)
		return
	}
	d, err := backend.NewDevice(dev, backend.BackendKernel)
	if err != nil {
		log.Printf("error creating device: %v\n", err)
		return
	}
	watchDevice(d)
}
