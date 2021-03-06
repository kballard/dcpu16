package main

import (
	"flag"
	"fmt"
	"github.com/kballard/dcpu16/dcpu"
	"github.com/kballard/dcpu16/dcpu/core"
	"github.com/kballard/termbox-go"
	"io/ioutil"
	"os"
)

var requestedRate dcpu.ClockRate = dcpu.DefaultClockRate
var printRate *bool = flag.Bool("printRate", false, "Print the effective clock rate at termination")
var screenRefreshRate dcpu.ClockRate = dcpu.DefaultScreenRefreshRate
var littleEndian *bool = flag.Bool("littleEndian", false, "Interpret the input file as little endian")

func main() {
	// command-line flags
	flag.Var(&requestedRate, "rate", "Clock rate to run the machine at")
	flag.Var(&screenRefreshRate, "screenRefreshRate", "Clock rate to refresh the screen at")
	// update usage
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] program\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	program := flag.Arg(0)
	data, err := ioutil.ReadFile(program)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Interpret the file as Words
	words := make([]core.Word, len(data)/2)
	for i := 0; i < len(data)/2; i++ {
		b1, b2 := core.Word(data[i*2]), core.Word(data[i*2+1])
		var w core.Word
		if *littleEndian {
			w = b2<<8 + b1
		} else {
			w = b1<<8 + b2
		}
		words[i] = w
	}

	// Set up a machine
	machine := new(dcpu.Machine)
	machine.Video.RefreshRate = screenRefreshRate
	if err := machine.State.LoadProgram(words, 0); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := machine.Start(requestedRate); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// convert termbox event polling into a channel
	events := make(chan termbox.Event)
	go func() {
		for {
			events <- termbox.PollEvent()
		}
	}()
	var effectiveRate dcpu.ClockRate
	printErr := func(err error) {
		fmt.Fprintln(os.Stderr, err)
		machine.State.Ram.DumpMemory(os.Stderr, []int{int(machine.State.PC())})
		os.Exit(1)
	}
	// now wait for keyboard events
loop:
	for {
		select {
		case evt := <-events:
			if evt.Type == termbox.EventKey {
				if evt.Key == termbox.KeyCtrlC {
					effectiveRate = machine.EffectiveClockRate()
					if err := machine.Stop(); err != nil {
						printErr(err)
					}
					break loop
				}
				// else pass it to the keyboard
				if evt.Ch == 0 {
					// it's a key constant
					key := evt.Key
					if r, ok := keymapTermboxKeyToRune[key]; ok {
						machine.Keyboard.RegisterKeyTyped(r)
					} else if k, ok := keymapTermboxKeyToKey[key]; ok {
						machine.Keyboard.RegisterKeyPressed(k)
						machine.Keyboard.RegisterKeyReleased(k)
					}
				} else {
					ch := evt.Ch
					if r, ok := keymapRuneToRune[evt.Ch]; ok {
						ch = r
					}
					machine.Keyboard.RegisterKeyTyped(ch)
				}
			}
		case err := <-machine.ErrorC:
			machine.Stop() // unlike HasError(), ErrorC doesn't shut down the machine
			printErr(err)
		}
	}
	if *printRate {
		fmt.Printf("Effective clock rate: %s\n", effectiveRate)
	}
}
