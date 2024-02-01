package main

import (
	"fmt"
	"runtime"
)

const (
	AppName         = "bindata"
	AppVersionMajor = 3
	AppVersionMinor = 1
)

// AppVersionRev revision part of the program version.
// This will be set automatically at build time like so:
//
//	go build -ldflags "-X main.AppVersionRev `date -u +%s`"
var AppVersionRev string

func Version() string {
	if len(AppVersionRev) == 0 {
		AppVersionRev = "3"
	}

	return fmt.Sprintf("%s %d.%d.%s (Go runtime %s).\nCopyright (c) 2010-2013, Jim Teeuwen.",
		AppName, AppVersionMajor, AppVersionMinor, AppVersionRev, runtime.Version())
}
