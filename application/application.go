package main

import (
	_ "os"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	initDao()
	initController()
}
