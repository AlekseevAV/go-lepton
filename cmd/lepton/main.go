// Copyright 2015 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime/pprof"

	// "github.com/israelshirk/go-lepton/leptontest"
	"github.com/maruel/interrupt"
	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/conn/spi/spireg"
	"periph.io/x/periph/devices/lepton"
	"periph.io/x/periph/host"
	"periph.io/x/periph/devices/lepton/image14bit"
)

func mainImpl() error {
	cpuprofile := flag.String("cpuprofile", "", "dump CPU profile in file")
	port := flag.Int("port", 8010, "http port to listen on")
	verbose := flag.Bool("verbose", false, "enable log output")
	i2cName := flag.String("i2c", "", "I²C bus to use")
	spiName := flag.String("spi", "", "SPI bus to use")
	flag.Parse()

	if len(flag.Args()) != 0 {
		return fmt.Errorf("unexpected argument: %s", flag.Args())
	}

	if !*verbose {
		log.SetOutput(ioutil.Discard)
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			return err
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	interrupt.HandleCtrlC()

	if _, err := host.Init(); err != nil {
		return err
	}

	var err error

	spiBus, err := spireg.Open(*spiName)
	if err != nil {
		return err
	}
	defer spiBus.Close()
	i2cBus, err := i2creg.Open(*i2cName)
	if err != nil {
		return err
	}
	defer i2cBus.Close()

	dev, err := lepton.New(spiBus, i2cBus)

	if err != nil {
		return fmt.Errorf("%s\nIf testing without hardware, use -fake to simulate a camera", err)
	}

	c := make(chan *lepton.Frame, 9*60)

	// Lepton reader loop.
	go func() {
		for {
			b := lepton.Frame{Gray14: image14bit.NewGray14(dev.Bounds())}
			// Keep this loop busy to not lose sync on SPI.
			err := dev.NextFrame(&b)
			if err != nil {
				log.Printf("%v", err)
			}
			c <- &b
		}
	}()

	w := StartWebServer(*port)
	go func() {
		for {
			w.AddImg(<-c)
		}
	}()

	fmt.Printf("\n")
	return watchFile()
}

func main() {
	if err := mainImpl(); err != nil {
		fmt.Fprintf(os.Stderr, "\nlepton: %s.\n", err)
		os.Exit(1)
	}
}
