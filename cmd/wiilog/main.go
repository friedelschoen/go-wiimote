package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/friedelschoen/go-wiimote"
	"github.com/friedelschoen/go-wiimote/driver"
	"github.com/friedelschoen/go-wiimote/pkg/discover"
)

var (
	openIf = flag.String("features", "", "features to use")
)

type eventBlock struct {
	Type      string        `json:"type"`
	Event     wiimote.Event `json:"event"`
	Id        string        `json:"id"`
	Timestamp time.Time     `json:"timestamp"`
	Feature   string        `json:"feature"`
}

func watchDevice(dev wiimote.Device, mu *sync.Mutex) {
	fmt.Printf("new device: %s\n", dev.String())
	time.Sleep(100 * time.Millisecond)
	var ifs wiimote.FeatureKind
	ifs |= wiimote.FeatureCore
	for name := range strings.SplitSeq(*openIf, ",") {
		switch name {
		case "accel":
			ifs |= wiimote.FeatureAccel
		case "bb", "balanceboard":
			ifs |= wiimote.FeatureBalanceBoard
		case "cc", "classiccontroller":
			ifs |= wiimote.FeatureClassicController
		case "drums":
			ifs |= wiimote.FeatureDrums
		case "guitar":
			ifs |= wiimote.FeatureGuitar
		case "ir":
			ifs |= wiimote.FeatureIR
		case "mp", "motionplus":
			ifs |= wiimote.FeatureMotionPlus
		case "nunchuck":
			ifs |= wiimote.FeatureNunchuck
		case "pc", "procontroller":
			ifs |= wiimote.FeatureProController
		}
	}
	if err := dev.OpenFeatures(ifs, true); err != nil {
		fmt.Fprintf(os.Stderr, "error: unable to open device: %s", err)
	}

	var block eventBlock
	for {
		ev, err := dev.Wait(-1)
		if err != nil {
			log.Printf("unable to poll event: %v\n", err)
		}
		if _, ok := ev.(*wiimote.EventGone); ok {
			return
		}

		block.Type = fmt.Sprintf("%T", ev)
		block.Event = ev
		block.Id = dev.Syspath()
		block.Timestamp = ev.Timestamp()
		if ev.Feature() != nil {
			block.Feature = ev.Feature().Kind().String()
		}
		b, err := json.Marshal(block)
		if err != nil {
			log.Printf("unable to encode event: %v\n", b)
		}
		mu.Lock()
		os.Stdout.Write(b)
		os.Stdout.WriteString("\n")
		mu.Unlock()
	}
}

func main() {
	flag.Parse()

	monitor, err := discover.NewWiimoteMonitor()
	if err != nil {
		log.Fatalln("error: ", err)
	}

	fmt.Println("waiting for devices...")
	var mu sync.Mutex
	for {
		dev, err := monitor.Wait(-1)
		if err != nil || dev == nil {
			log.Printf("error while polling: %v\n", err)
			continue
		}
		d, err := driver.NewDevice(dev, driver.BackendKernel)
		if err != nil {
			log.Printf("error creating device: %v\n", err)
			continue
		}
		go watchDevice(d, &mu)
	}
}
