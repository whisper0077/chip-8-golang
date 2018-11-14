package main

import (
	"flag"
	"io/ioutil"
	"os"
	"runtime"

	e "github.com/tuboc/chip8/emulator"
)

var filename = flag.String("f", "", "chip8 image file path")
var stepMode = flag.Bool("s", false, "start with stepMode")

func init() {
	runtime.LockOSThread()
}

func main() {
	flag.Parse()

	f, _ := os.Open(*filename)
	binary, _ := ioutil.ReadAll(f)

	emu := e.NewEmulator(binary, *stepMode)
	emu.Run()
}
