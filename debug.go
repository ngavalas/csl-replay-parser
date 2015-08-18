package main

import "fmt"

var debugMode = false

func InitDebugMode(debug bool) {
	debugMode = debug
}

func DebugPrint(format string, args ...interface{}) {
	if debugMode {
		fmt.Printf(format, args...)
	}
}
